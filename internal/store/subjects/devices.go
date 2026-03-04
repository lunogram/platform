package subjects

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

type Devices []Device

func (d *Devices) Scan(value any) error {
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
	return json.Unmarshal(buf, d)
}

func (d Devices) Value() (driver.Value, error) {
	return json.Marshal(d)
}

func (d Devices) HasPushDevice() bool {
	for _, device := range d {
		if device.HasPushToken() {
			return true
		}
	}
	return false
}

type Device struct {
	ID         uuid.UUID `db:"id" json:"id"`
	ProjectID  uuid.UUID `db:"project_id" json:"project_id"`
	UserID     uuid.UUID `db:"user_id" json:"user_id"`
	DeviceID   string    `db:"device_id" json:"device_id"`
	Token      *string   `db:"token" json:"token"`
	OS         *string   `db:"os" json:"os"`
	OSVersion  *string   `db:"os_version" json:"os_version"`
	Model      *string   `db:"model" json:"model"`
	AppBuild   *string   `db:"app_build" json:"app_build"`
	AppVersion *string   `db:"app_version" json:"app_version"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// HasPushToken returns true if the device has a non-null token
func (d *Device) HasPushToken() bool {
	return d.Token != nil && *d.Token != ""
}

func NewDevicesStore(db store.DB) *DevicesStore {
	return &DevicesStore{db: db}
}

type DevicesStore struct {
	db store.DB
}

func (s *DevicesStore) CreateDevice(ctx context.Context, device Device) (uuid.UUID, error) {
	stmt := `
	INSERT INTO devices (project_id, user_id, device_id, token, os, model)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		device.ProjectID,
		device.UserID,
		device.DeviceID,
		device.Token,
		device.OS,
		device.Model,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *DevicesStore) ListDevicesByUser(ctx context.Context, projectID, userID uuid.UUID) (Devices, error) {
	query := `
	SELECT id, project_id, user_id, device_id, token, os, os_version, model, app_build, app_version, created_at, updated_at
	FROM devices
	WHERE project_id = $1 AND user_id = $2
	AND token IS NOT NULL AND token != ''`

	var devices Devices
	err := s.db.SelectContext(ctx, &devices, query, projectID, userID)
	if err != nil {
		return nil, err
	}

	return devices, nil
}
