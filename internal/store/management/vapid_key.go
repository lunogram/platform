package management

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

const DefaultVapidKeyName = "default"

type VapidKey struct {
	ID         uuid.UUID `db:"id"`
	Name       string    `db:"name"`
	PublicKey  string    `db:"public_key"`
	PrivateKey string    `db:"private_key"`
	CreatedAt  time.Time `db:"created_at"`
}

func NewVapidKeysStore(db store.DB) *VapidKeysStore {
	return &VapidKeysStore{
		db: db,
	}
}

type VapidKeysStore struct {
	db store.DB
}

func (s *VapidKeysStore) GetVapidKeyByName(ctx context.Context, name string) (*VapidKey, error) {
	var key VapidKey
	err := s.db.GetContext(ctx, &key, "SELECT id, name, public_key, private_key, created_at FROM vapid_keys WHERE name = $1 AND deleted_at IS NULL", name)
	if err != nil {
		return nil, err
	}

	return &key, nil
}

func (s *VapidKeysStore) CreateVapidKey(ctx context.Context, name string) error {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(
		ctx,
		"INSERT INTO vapid_keys (name, public_key, private_key) VALUES ($1, $2, $3)",
		name,
		publicKey,
		privateKey,
	)

	return err
}

func (s *VapidKeysStore) CreateVapidKeysIfNotExist(ctx context.Context) error {
	_, err := s.GetVapidKeyByName(ctx, DefaultVapidKeyName)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	err = s.CreateVapidKey(ctx, DefaultVapidKeyName)
	if err != nil {
		return err
	}

	return nil
}
