package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/caarlos0/env/v10"
	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/services/nexus/internal/cluster"
	"github.com/lunogram/platform/services/nexus/internal/cluster/consensus"
	"github.com/lunogram/platform/services/nexus/internal/cluster/leader"
	"github.com/lunogram/platform/services/nexus/internal/cluster/scheduler"
	"github.com/lunogram/platform/services/nexus/internal/config"
	managementv1 "github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management"
	publicv1 "github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/public"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/consumer"
	"github.com/lunogram/platform/services/nexus/internal/storage"
	"github.com/lunogram/platform/services/nexus/internal/store"
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

	if migrate {
		logger.Info("running database migrations...")
		return store.Migrate(conf.Store)
	}

	logger.Info("starting service...")
	logger.Info("initializing database")

	db, err := store.New(ctx, logger, conf.Store)
	if err != nil {
		return err
	}

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

	pub := pubsub.NewPublisher(jet)
	consumer.Serve(ctx, jet, logger, db)

	logger.Info("initializing cluster")

	scheduler := scheduler.NewController(ctx, logger, conf, db, pub)
	leader := leader.NewHandler(scheduler)
	consensus, err := consensus.NewCluster(ctx, logger, conf)
	if err != nil {
		return err
	}

	_, err = cluster.NewNode(ctx, logger, conf, consensus, leader)
	if err != nil {
		return err
	}

	logger.Info("starting http servers")

	mgmt, err := managementv1.NewServer(ctx, logger, conf, db, bucket, pub)
	if err != nil {
		return err
	}

	logger.Info("serving management http server")
	go mgmt.Serve(ctx, conf.ManagementServiceAddress)

	public, err := publicv1.NewServer(ctx, logger, conf, db, bucket, pub)
	if err != nil {
		return err
	}

	logger.Info("serving public http server")
	go public.Serve(ctx, conf.PublicServiceAddress)

	logger.Info("service up and running!")
	ctx.AwaitKillSignal()
	return nil
}
