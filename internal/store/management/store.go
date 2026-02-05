package management

import (
	"fmt"

	"github.com/cloudproud/graceful"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/store"
	"go.uber.org/zap"
)

type Config struct {
	URI string `env:"POSTGRES_URI" envDefault:"postgres://postgres:password@postgres:5432/postgres?sslmode=disable"`
}

func New(ctx graceful.Context, logger *zap.Logger, config Config) (*sqlx.DB, error) {
	logger.Info("connecting to management PostgreSQL database")

	db, err := sqlx.Connect("pgx", config.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to management database: %w", err)
	}

	ctx.Closer(func() {
		logger.Info("received close signal, closing management store client")

		err := db.Close()
		if err != nil {
			logger.Error("failed to close management database connection", zap.Error(err))
		}
	})

	return db, nil
}

func NewState(db store.DB) *State {
	return &State{
		AdminsStore:        NewAdminsStore(db),
		ProjectsStore:      NewProjectsStore(db),
		CampaignsStore:     NewCampaignsStore(db),
		ProvidersStore:     NewProvidersStore(db),
		TemplatesStore:     NewTemplatesStore(db),
		SubscriptionsStore: NewSubscriptionsStore(db),
		OrganizationsStore: NewOrganizationsStore(db),
		TagsStore:          NewTagsStore(db),
		LocalesStore:       NewLocalesStore(db),
		DocumentsStore:     NewDocumentsStore(db),
		AuthStore:          NewAuthStore(db),
	}
}

type State struct {
	*AdminsStore
	*ProjectsStore
	*CampaignsStore
	*ProvidersStore
	*TemplatesStore
	*SubscriptionsStore
	*OrganizationsStore
	*TagsStore
	*LocalesStore
	*DocumentsStore
	*AuthStore
}
