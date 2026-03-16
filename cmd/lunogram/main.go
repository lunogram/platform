package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/caarlos0/env/v10"
	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/cluster"
	"github.com/lunogram/platform/internal/cluster/consensus"
	"github.com/lunogram/platform/internal/cluster/leader"
	"github.com/lunogram/platform/internal/cluster/scheduler"
	"github.com/lunogram/platform/internal/config"
	v1 "github.com/lunogram/platform/internal/http/controllers/v1"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
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

	if migrate || conf.DatabaseMigrate {
		logger.Info("running database migrations...")
		logger.Info("running management migration")
		if err := management.Migrate(conf.Store.ManagementURI); err != nil {
			return fmt.Errorf("management migration failed: %w", err)
		}

		logger.Info("running subjects migration")
		if err := subjects.Migrate(conf.Store.SubjectsURI); err != nil {
			return fmt.Errorf("subjects migration failed: %w", err)
		}

		logger.Info("running journey migration")
		if err := journey.Migrate(conf.Store.JourneyURI); err != nil {
			return fmt.Errorf("journey migration failed: %w", err)
		}

		logger.Info("running rbac migration")
		if err := rbac.Migrate(conf.RBAC); err != nil {
			return fmt.Errorf("rbac migration failed: %w", err)
		}
		if migrate {
			return nil
		}
	}

	logger.Info("starting service...")
	logger.Info("initializing database")

	db, err := store.New(ctx, logger, conf.Store)
	if err != nil {
		return err
	}

	managementStore := management.NewState(db.Management)
	usersStore := subjects.NewState(db.Subjects)
	journeyStore := journey.NewState(db.Journey)

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

	err = consumer.Bootstrap(ctx, logger, jet, consumer.Namespace(conf.Nats.Namespace))
	if err != nil {
		return err
	}

	logger.Info("initializing provider registry")

	var providersRegisrtry *providers.Registry
	if slices.Contains(conf.Modules, "wasm") {
		logger.Info("wasm module is enabled, initializing wasm provider registry")

		providersRegisrtry, err := providers.NewRegistry(ctx, conf.WASM, logger)
		if err != nil {
			return err
		}
		defer providersRegisrtry.Close(ctx)
	} else {
		logger.Info("wasm module is disabled, skipping wasm provider registry initialization")
	}

	logger.Info("initializing action registry")

	actionRegistry, err := actions.NewRegistry(ctx, conf.WASM, logger)
	if err != nil {
		return err
	}
	defer actionRegistry.Close(ctx)

	pub := pubsub.NewPublisher(jet, conf.Nats.Namespace)
	req := pubsub.NewCaller(jet, conf.Nats.Namespace)
	ns := consumer.Namespace(conf.Nats.Namespace)
	consumer.Serve(ctx, jet, logger, ns, db, managementStore, usersStore, journeyStore, providersRegisrtry, actionRegistry, req, conf.PublicURL, conf)

	if slices.Contains(conf.Modules, "scheduler") {
		logger.Info("starting scheduler")

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
	} else {
		logger.Info("scheduler module is disabled, skipping cluster initialization")
	}

	logger.Info("initializing rbac engine")

	rbacEngine, err := rbac.NewEngine(ctx, conf.RBAC)
	if err != nil {
		return fmt.Errorf("failed to initialize rbac engine: %w", err)
	}
	defer rbacEngine.Close()

	if slices.Contains(conf.Modules, "http") {
		logger.Info("starting http server")

		server, err := v1.NewServer(ctx, logger, conf, db, bucket, jet, pub, req, providersRegisrtry, actionRegistry, rbacEngine)
		if err != nil {
			return err
		}

		logger.Info("serving http server")
		go server.Serve(ctx, conf.HTTPAddress)
	} else {
		logger.Info("http module is disabled, skipping http server startup")
	}

	logger.Info("service up and running!")
	ctx.AwaitKillSignal()
	return nil
}
