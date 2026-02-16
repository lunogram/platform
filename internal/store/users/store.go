package users

import (
	"fmt"

	"github.com/cloudproud/graceful"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/store"
	"go.uber.org/zap"
)

type Config struct {
	URI string `env:"USERS_POSTGRES_URI" envDefault:"postgres://postgres:password@postgres:5432/users?sslmode=disable"`
}

func New(ctx graceful.Context, logger *zap.Logger, config Config) (*sqlx.DB, error) {
	logger.Info("connecting to users PostgreSQL database")

	db, err := sqlx.Connect("pgx", config.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to users database: %w", err)
	}

	ctx.Closer(func() {
		logger.Info("received close signal, closing users store client")

		err := db.Close()
		if err != nil {
			logger.Error("failed to close users database connection", zap.Error(err))
		}
	})

	return db, nil
}

func NewState(db store.DB) *State {
	return &State{
		UsersStore:   NewUsersStore(db),
		EventsStore:  NewEventsStore(db),
		DevicesStore: NewDevicesStore(db),
		ListsStore:   NewListsStore(db),
		RulesStore:   NewRulesStore(db),
	}
}

type State struct {
	*UsersStore
	*EventsStore
	*DevicesStore
	*ListsStore
	*RulesStore
}
