package management

import (
	"github.com/SherClockHolmes/webpush-go"
	"github.com/lunogram/platform/internal/store"
)

type VapidKey struct {
	ID         string `db:"id"`
	Name       string `db:"name"`
	PublicKey  string `db:"public_key"`
	PrivateKey string `db:"private_key"`
	CreatedAt  string `db:"created_at"`
}

func NewVapidKeysStore(db store.DB) *VapidKeysStore {
	return &VapidKeysStore{
		db: db,
	}
}

type VapidKeysStore struct {
	db store.DB
}

func (s *VapidKeysStore) GetVapidKeyByName(name string) (*VapidKey, error) {
	var key VapidKey
	err := s.db.Get(&key, "SELECT * FROM vapid_keys WHERE name = $1 AND deleted_at IS NULL", name)
	if err != nil {
		return nil, err
	}

	return &key, nil
}

func (s *VapidKeysStore) CreateVapidKey(name string) error {
	privateKey, pblicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		"INSERT INTO vapid_keys (name, public_key, private_key) VALUES ($1, $2, $3)",
		name,
		pblicKey,
		privateKey,
	)

	return err
}

func (s *VapidKeysStore) CreateVapidKeysIfNotExist() error {
	vapidKeyName := "default"
	_, err := s.GetVapidKeyByName(vapidKeyName)
	if err == nil {
		return nil
	}

	err = s.CreateVapidKey(vapidKeyName)
	if err != nil {
		return err
	}

	return nil
}
