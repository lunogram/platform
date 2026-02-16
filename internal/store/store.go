package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/jmoiron/sqlx"
)

// Config contains database connection settings for all stores.
type Config struct {
	ManagementURI string `env:"POSTGRES_URI" envDefault:"postgres://postgres:password@postgres:5432/postgres?sslmode=disable"`
	UsersURI      string `env:"USERS_POSTGRES_URI" envDefault:"postgres://postgres:password@postgres:5432/users?sslmode=disable"`
	JourneyURI    string `env:"JOURNEY_POSTGRES_URI" envDefault:"postgres://postgres:password@postgres:5432/journey?sslmode=disable"`
}

var ErrNoRows = sql.ErrNoRows

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
