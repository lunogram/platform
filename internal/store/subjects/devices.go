package subjects

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

type PushConfigType string

const (
	PushConfigTypeFCM     PushConfigType = "fcm"
	PushConfigTypeAPNs    PushConfigType = "apns"
	PushConfigTypeWebPush PushConfigType = "webpush"
)

type PushConfigKeys struct {
	Auth   string `json:"auth"`
	P256dh string `json:"p256dh"`
}

type PushConfig struct {
	Type           PushConfigType  `json:"type"`
	Token          string          `json:"token,omitempty"`          // fcm / apns
	Endpoint       string          `json:"endpoint,omitempty"`       // webpush
	ExpirationTime *time.Time      `json:"expirationTime,omitempty"` // webpush
	Keys           *PushConfigKeys `json:"keys,omitempty"`           // webpush
}

func (p *PushConfig) Scan(src any) error {
	b, ok := src.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, p)
}

func (p PushConfig) Value() (driver.Value, error) {
	return json.Marshal(p)
}

type Devices []Device

type PublicDevice struct {
	ID         uuid.UUID `db:"id" json:"id"`
	DeviceID   string    `db:"device_id" json:"device_id"`
	OS         *string   `db:"os" json:"os,omitempty"`
	OSVersion  *string   `db:"os_version" json:"os_version,omitempty"`
	Model      *string   `db:"model" json:"model,omitempty"`
	AppBuild   *string   `db:"app_build" json:"app_build,omitempty"`
	AppVersion *string   `db:"app_version" json:"app_version,omitempty"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

type PublicDevices []PublicDevice

func (pd PublicDevices) OAPI() []oapi.UserDevice {
	results := make([]oapi.UserDevice, len(pd))
	for i, device := range pd {
		results[i] = device.OAPI()
	}
	return results
}

func (d *PublicDevice) OAPI() oapi.UserDevice {
	return oapi.UserDevice{
		Id:         d.ID,
		DeviceId:   d.DeviceID,
		Os:         d.OS,
		OsVersion:  d.OSVersion,
		Model:      d.Model,
		AppBuild:   d.AppBuild,
		AppVersion: d.AppVersion,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

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

func (d Devices) HasPushConfig() bool {
	for _, device := range d {
		if device.PushConfig != nil {
			return true
		}
	}
	return false
}

type Device struct {
	ID         uuid.UUID   `db:"id" json:"id"`
	ProjectID  uuid.UUID   `db:"project_id" json:"project_id"`
	UserID     uuid.UUID   `db:"user_id" json:"user_id"`
	DeviceID   string      `db:"device_id" json:"device_id"`
	PushConfig *PushConfig `db:"push_config" json:"push_config,omitempty"`
	OS         *string     `db:"os" json:"os"`
	OSVersion  *string     `db:"os_version" json:"os_version"`
	Model      *string     `db:"model" json:"model"`
	AppBuild   *string     `db:"app_build" json:"app_build"`
	AppVersion *string     `db:"app_version" json:"app_version"`
	CreatedAt  time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time   `db:"updated_at" json:"updated_at"`
}

func (d *Device) IsFCM() bool {
	return d.PushConfig != nil && d.PushConfig.Type == PushConfigTypeFCM
}

func (d *Device) IsAPNs() bool {
	return d.PushConfig != nil && d.PushConfig.Type == PushConfigTypeAPNs
}

func (d *Device) IsWebPush() bool {
	return d.PushConfig != nil && d.PushConfig.Type == PushConfigTypeWebPush
}

func (d *Device) OAPI() oapi.UserDevice {
	return oapi.UserDevice{
		Id:         d.ID,
		DeviceId:   d.DeviceID,
		Os:         d.OS,
		OsVersion:  d.OSVersion,
		Model:      d.Model,
		AppBuild:   d.AppBuild,
		AppVersion: d.AppVersion,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

func (d Devices) OAPI() []oapi.UserDevice {
	results := make([]oapi.UserDevice, len(d))
	for i, device := range d {
		results[i] = device.OAPI()
	}
	return results
}

func NewDevicesStore(db store.DB) *DevicesStore {
	return &DevicesStore{db: db}
}

type DevicesStore struct {
	db store.DB
}

func (s *DevicesStore) CreateDevice(ctx context.Context, device Device) (uuid.UUID, error) {
	stmt := `
	INSERT INTO devices (project_id, user_id, device_id, push_config, os, model)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		device.ProjectID,
		device.UserID,
		device.DeviceID,
		device.PushConfig,
		device.OS,
		device.Model,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *DevicesStore) ListDevicesByUser(ctx context.Context, projectID, userID uuid.UUID) (PublicDevices, error) {
	query := `
	SELECT id, project_id, user_id, device_id, os, os_version, model, app_build, app_version, created_at, updated_at
	FROM devices
	WHERE project_id = $1 AND user_id = $2
	AND deleted_at IS NULL`

	var devices PublicDevices
	err := s.db.SelectContext(ctx, &devices, query, projectID, userID)
	if err != nil {
		return nil, err
	}

	return devices, nil
}

func (s *DevicesStore) ListDevicesByUserWithPushConfig(ctx context.Context, projectID, userID uuid.UUID) (Devices, error) {
	query := `
	SELECT id, project_id, user_id, device_id, push_config, os, os_version, model, app_build, app_version, created_at, updated_at
	FROM devices
	WHERE project_id = $1 AND user_id = $2
	AND push_config IS NOT NULL
	AND deleted_at IS NULL`

	var devices Devices
	err := s.db.SelectContext(ctx, &devices, query, projectID, userID)
	if err != nil {
		return nil, err
	}

	return devices, nil
}

func (s *DevicesStore) DeleteDevice(ctx context.Context, projectID, deviceID uuid.UUID) error {
	query := `
	UPDATE devices
	SET deleted_at = NOW()
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID, deviceID)
	return err
}

func (s *DevicesStore) UpdateDevicePushConfig(ctx context.Context, projectID, userID uuid.UUID, deviceID string, config PushConfig) error {
	query := `
	UPDATE devices
	SET push_config = $1, updated_at = NOW()
	WHERE project_id = $2 AND user_id = $3 AND device_id = $4
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, config, projectID, userID, deviceID)
	return err
}

func (s *DevicesStore) UpsertDevice(ctx context.Context, device Device) error {
	query := `
	INSERT INTO devices (project_id, user_id, device_id, push_config, os, os_version, model, app_build, app_version)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT (project_id, device_id, deleted_at)
	WHERE deleted_at IS NULL
	DO UPDATE SET
		user_id     = EXCLUDED.user_id,
		push_config = COALESCE(EXCLUDED.push_config, devices.push_config),
		os          = COALESCE(EXCLUDED.os, devices.os),
		os_version  = COALESCE(EXCLUDED.os_version, devices.os_version),
		model       = COALESCE(EXCLUDED.model, devices.model),
		app_build   = COALESCE(EXCLUDED.app_build, devices.app_build),
		app_version = COALESCE(EXCLUDED.app_version, devices.app_version),
		updated_at  = NOW()`

	_, err := s.db.ExecContext(ctx, query,
		device.ProjectID,
		device.UserID,
		device.DeviceID,
		device.PushConfig,
		device.OS,
		device.OSVersion,
		device.Model,
		device.AppBuild,
		device.AppVersion,
	)
	return err
}
