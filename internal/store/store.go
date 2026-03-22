package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/cloudproud/graceful"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Config contains database connection settings for all databases.
type Config struct {
	ManagementURI string `env:"POSTGRES_MANAGEMENT_URI" envDefault:"postgres://postgres:postgrespw@postgres:5432/management?sslmode=disable"`
	SubjectsURI   string `env:"POSTGRES_SUBJECTS_URI" envDefault:"postgres://postgres:postgrespw@postgres:5432/subjects?sslmode=disable"`
	JourneyURI    string `env:"POSTGRES_JOURNEY_URI" envDefault:"postgres://postgres:postgrespw@postgres:5432/journey?sslmode=disable"`
}

// Connections holds all database connections.
type Connections struct {
	Management *sqlx.DB
	Subjects   *sqlx.DB
	Journey    *sqlx.DB
}

// New creates database connections for all databases.
func New(ctx graceful.Context, logger *zap.Logger, config Config) (*Connections, error) {
	logger.Info("connecting to PostgreSQL databases")

	management, err := sqlx.Connect("pgx", config.ManagementURI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to management database: %w", err)
	}
	management.SetMaxOpenConns(25)
	management.SetMaxIdleConns(5)

	subjects, err := sqlx.Connect("pgx", config.SubjectsURI)
	if err != nil {
		management.Close()
		return nil, fmt.Errorf("failed to connect to subjects database: %w", err)
	}
	subjects.SetMaxOpenConns(25)
	subjects.SetMaxIdleConns(5)

	journey, err := sqlx.Connect("pgx", config.JourneyURI)
	if err != nil {
		management.Close()
		subjects.Close()
		return nil, fmt.Errorf("failed to connect to journey database: %w", err)
	}
	journey.SetMaxOpenConns(25)
	journey.SetMaxIdleConns(5)

	conns := &Connections{
		Management: management,
		Subjects:   subjects,
		Journey:    journey,
	}

	ctx.Closer(func() {
		logger.Info("received close signal, closing database connections")

		if err := management.Close(); err != nil {
			logger.Error("failed to close management database connection", zap.Error(err))
		}
		if err := subjects.Close(); err != nil {
			logger.Error("failed to close subjects database connection", zap.Error(err))
		}
		if err := journey.Close(); err != nil {
			logger.Error("failed to close journey database connection", zap.Error(err))
		}
	})

	return conns, nil
}

// Connect creates a single database connection with graceful shutdown.
func Connect(ctx graceful.Context, logger *zap.Logger, uri string) (*sqlx.DB, error) {
	logger.Info("connecting to PostgreSQL database", zap.String("uri", uri))

	db, err := sqlx.Connect("pgx", uri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Limit connection pool to prevent exhausting PostgreSQL max_connections,
	// especially during parallel test execution where many pools share one server.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	ctx.Closer(func() {
		logger.Info("received close signal, closing database connection")

		err := db.Close()
		if err != nil {
			logger.Error("failed to close database connection", zap.Error(err))
		}
	})

	return db, nil
}

var ErrNoRows = sql.ErrNoRows

// Migrate applies all migrations from the given filesystem to the database.
func Migrate(uri string, migrations fs.FS) error {
	fsDriver, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to load embedded migration: %w", err)
	}

	conn, err := sql.Open("pgx", uri)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	conn.SetMaxOpenConns(2)
	conn.SetMaxIdleConns(1)
	defer conn.Close()

	db, err := pgx.WithInstance(conn, &pgx.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration database instance: %w", err)
	}
	defer db.Close() //nolint:errcheck

	migrator, err := migrate.NewWithInstance("iofs", fsDriver, "pgx", db)
	if err != nil {
		return fmt.Errorf("failed to construct migrator: %w", err)
	}

	err = migrator.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	return nil
}

// DB is the common database interface used by all store implementations.
type DB interface {
	sqlx.Ext
	sqlx.ExtContext
	Get(dest any, query string, args ...any) error
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	Select(dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	NamedExec(query string, arg any) (sql.Result, error)
	NamedExecContext(ctx context.Context, query string, arg any) (sql.Result, error)
}

// Pagination contains limit and offset for paginated queries.
type Pagination struct {
	Limit  int
	Offset int
}

// JSONB is a generic wrapper for JSONB database columns.
type JSONB[T any] struct {
	Data T
}

// Scan implements sql.Scanner for reading from database.
func (j *JSONB[T]) Scan(value any) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSONB: expected []byte")
	}

	return json.Unmarshal(bytes, &j.Data)
}

// Value implements driver.Valuer for writing to database.
func (j JSONB[T]) Value() (driver.Value, error) {
	return json.Marshal(j.Data)
}

// MarshalRaw returns the JSONB data as a json.RawMessage pointer.
func (j *JSONB[T]) MarshalRaw() *json.RawMessage {
	if j == nil {
		return nil
	}

	bytes, err := json.Marshal(j.Data)
	if err != nil {
		return nil
	}

	return (*json.RawMessage)(&bytes)
}

// UUIDArray is a custom type for scanning PostgreSQL UUID arrays.
type UUIDArray []uuid.UUID

// Scan implements sql.Scanner for reading PostgreSQL UUID arrays.
func (u *UUIDArray) Scan(value any) error {
	if value == nil {
		*u = nil
		return nil
	}

	// PostgreSQL returns array_agg results as a string like "{uuid1,uuid2,...}"
	var str string
	switch v := value.(type) {
	case []byte:
		str = string(v)
	case string:
		str = v
	default:
		return fmt.Errorf("unsupported type for UUIDArray: %T", value)
	}

	// Handle empty array
	if str == "{}" || str == "" {
		*u = []uuid.UUID{}
		return nil
	}

	// Remove braces and split
	str = strings.Trim(str, "{}")
	if str == "" {
		*u = []uuid.UUID{}
		return nil
	}

	parts := strings.Split(str, ",")
	result := make([]uuid.UUID, len(parts))
	for i, p := range parts {
		id, err := uuid.Parse(p)
		if err != nil {
			return fmt.Errorf("failed to parse UUID at index %d: %w", i, err)
		}
		result[i] = id
	}

	*u = result
	return nil
}

// DataType represents the type of a data field.
type DataType string

const (
	DataTypeString  DataType = "string"
	DataTypeNumber  DataType = "number"
	DataTypeBoolean DataType = "boolean"
	DataTypeObject  DataType = "object"
	DataTypeArray   DataType = "array"
)

// JourneyVersionStepChild represents a connection between journey steps.
type JourneyVersionStepChild struct {
	VersionID        string           `db:"version_id" json:"version_id"`
	ParentExternalID string           `db:"parent_external_id" json:"parent_external_id"`
	ChildExternalID  string           `db:"child_external_id" json:"child_external_id"`
	Path             *string          `db:"path" json:"path,omitempty"`
	Data             *json.RawMessage `db:"data" json:"data,omitempty"`
}

// JourneyVersionStepChildren is a slice of JourneyVersionStepChild.
type JourneyVersionStepChildren []JourneyVersionStepChild

// Scan implements sql.Scanner for reading JSON from database.
func (steps *JourneyVersionStepChildren) Scan(value any) error {
	if value == nil {
		return nil
	}
	var buf []byte
	switch v := value.(type) {
	case []byte:
		buf = v
	case string:
		buf = []byte(v)
	}
	return json.Unmarshal(buf, steps)
}
