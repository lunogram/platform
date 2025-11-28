package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/caarlos0/env/v10"
	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http"
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

	conf := config.Service{}
	err = env.Parse(&conf)
	if err != nil {
		return err
	}

	if migrate {
		logger.Info("running database migrations...")
		return store.Migrate(conf.Store)
	}

	logger.Info("starting service...")
	logger.Info("connecting to database")

	db, err := store.Connect(ctx, conf.Store)
	if err != nil {
		return err
	}

	logger.Info("starting http server")

	srv, err := http.NewServer(ctx, logger, conf, db)
	if err != nil {
		return err
	}

	logger.Info("serving http server")
	go srv.Serve(ctx, conf.Address)

	logger.Info("service up and running!")
	ctx.AwaitKillSignal()
	return nil
}
