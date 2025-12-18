package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudproud/graceful"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var ErrNoRows = sql.ErrNoRows

type Config struct {
	URI string `env:"POSTGRES_URI" envDefault:"postgres://postgres:password@postgres:5432/postgres?sslmode=disable"`
}

func New(ctx graceful.Context, logger *zap.Logger, service Config) (*sqlx.DB, error) {
	logger.Info("connecting to PostgreSQL database")

	db, err := sqlx.Connect("pgx", service.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	ctx.Closer(func() {
		logger.Info("received close signal, closing store client")

		err := db.Close()
		if err != nil {
			logger.Error("failed to close database connection", zap.Error(err))
		}
	})

	return db, nil
}

type DB interface {
	sqlx.Ext
	sqlx.ExtContext
	Get(dest interface{}, query string, args ...interface{}) error
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	NamedExec(query string, arg interface{}) (sql.Result, error)
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
}

func NewStores(db DB) *Stores {
	return &Stores{
		AdminsStore:        NewAdminsStore(db),
		ProjectsStore:      NewProjectsStore(db),
		CampaignsStore:     NewCampaignsStore(db),
		ProvidersStore:     NewProvidersStore(db),
		TemplatesStore:     NewTemplatesStore(db),
		UsersStore:         NewUsersStore(db),
		SubscriptionsStore: NewSubscriptionsStore(db),
		JourneysStore:      NewJourneysStore(db),
		OrganizationsStore: NewOrganizationsStore(db),
		DevicesStore:       NewDevicesStore(db),
		TagsStore:          NewTagsStore(db),
		LocalesStore:       NewLocalesStore(db),
		ListsStore:         NewListsStore(db),
		DocumentsStore:     NewDocumentsStore(db),
		EventsStore:        NewEventsStore(db),
		AuthStore:          NewAuthStore(db),
	}
}

type Stores struct {
	*AdminsStore
	*ProjectsStore
	*CampaignsStore
	*ProvidersStore
	*TemplatesStore
	*UsersStore
	*SubscriptionsStore
	*JourneysStore
	*OrganizationsStore
	*DevicesStore
	*TagsStore
	*LocalesStore
	*ListsStore
	*DocumentsStore
	*EventsStore
	*AuthStore
}

type Pagination struct {
	Limit  int
	Offset int
}

type JSONB[T any] struct {
	Data T
}

// Scan implements sql.Scanner for reading from database
func (j *JSONB[T]) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSONB: expected []byte")
	}

	return json.Unmarshal(bytes, &j.Data)
}

// Value implements driver.Valuer for writing to database
func (j JSONB[T]) Value() (driver.Value, error) {
	return json.Marshal(j.Data)
}

type DataType string

const (
	DataTypeString DataType = "string"
	DataTypeNumber DataType = "number"
	DataTypeBool   DataType = "bool"
	DataTypeObject DataType = "object"
	DataTypeArray  DataType = "array"
)
