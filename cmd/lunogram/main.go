package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/caarlos0/env/v10"
	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/cluster"
	"github.com/lunogram/platform/internal/cluster/consensus"
	"github.com/lunogram/platform/internal/cluster/leader"
	"github.com/lunogram/platform/internal/cluster/scheduler"
	"github.com/lunogram/platform/internal/config"
	v1 "github.com/lunogram/platform/internal/http/controllers/v1"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"go.uber.org/zap"
)

var migrate bool

func init() {
	flag.BoolVar(&migrate, "migrate", false, "flag indicating whether the service should run migrations and exit")
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()
	ctx := graceful.NewContext(context.Background())

	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	defer logger.Sync() //nolint:errcheck

	conf := config.Node{}
	err = env.Parse(&conf)
	if err != nil {
		return err
	}

	managementConfig := management.Config{URI: conf.Store.ManagementURI}

	if migrate {
		logger.Info("running database migrations...")
		return management.Migrate(managementConfig)
	}

	if conf.DatabaseMigrate {
		logger.Info("running database migrations...")
		if err := management.Migrate(managementConfig); err != nil {
			return fmt.Errorf("auto-migrate failed: %w", err)
		}
	}

	logger.Info("starting service...")
	logger.Info("initializing database")

	managementDB, err := management.New(ctx, logger, managementConfig)
	if err != nil {
		return err
	}

	managementStore := management.NewState(managementDB)
	usersStore := users.NewState(managementDB)
	journeyStore := journey.NewState(managementDB)

	logger.Info("initializing block storage")

	bucket, err := storage.New(conf.Storage)
	if err != nil {
		return err
	}

	logger.Info("initializing pubsub...")

	jet, err := pubsub.New(ctx, conf)
	if err != nil {
		return err
	}

	err = consumer.Bootstrap(ctx, logger, jet)
	if err != nil {
		return err
	}

	logger.Info("initializing provider registry")

	registry, err := providers.NewRegistry(ctx, conf.WASM, logger)
	if err != nil {
		return err
	}
	defer registry.Close(ctx)

	pub := pubsub.NewPublisher(jet)
	consumer.Serve(ctx, jet, logger, managementDB, managementStore, usersStore, journeyStore, registry)

	logger.Info("initializing cluster")

	sched := scheduler.NewController(ctx, logger, conf, journeyStore, pub)
	lead := leader.NewHandler(sched)
	cons, err := consensus.NewCluster(ctx, logger, conf)
	if err != nil {
		return err
	}

	_, err = cluster.NewNode(ctx, logger, conf, cons, lead)
	if err != nil {
		return err
	}

	logger.Info("starting http server")

	server, err := v1.NewServer(ctx, logger, conf, managementDB, bucket, pub, registry)
	if err != nil {
		return err
	}

	logger.Info("serving http server")
	go server.Serve(ctx, conf.HTTPAddress)

	logger.Info("service up and running!")
	ctx.AwaitKillSignal()
	return nil
}
