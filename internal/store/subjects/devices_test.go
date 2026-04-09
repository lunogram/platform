package subjects

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUpsertDeviceRevivesSoftDeletedDevice(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()

	userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{{Source: "anonymous", ExternalID: "anon_device_revive"}})
	require.NoError(t, err)

	deviceUUID, err := db.CreateDevice(ctx, Device{
		ProjectID: projectID,
		UserID:    userID,
		DeviceID:  "device-revive",
		Token:     ptr("token-revive"),
		OS:        ptr("ios"),
		Model:     ptr("iPhone"),
	})
	require.NoError(t, err)

	err = db.DeleteDevice(ctx, projectID, deviceUUID)
	require.NoError(t, err)

	devices, err := db.ListDevicesByUser(ctx, projectID, userID)
	require.NoError(t, err)
	require.Len(t, devices, 0)

	upsertedID, err := db.UpsertDevice(ctx, Device{
		ProjectID:  projectID,
		UserID:     userID,
		DeviceID:   "device-revive",
		Token:      ptr("token-revive-new"),
		OS:         ptr("ios"),
		OSVersion:  ptr("18.0"),
		Model:      ptr("iPhone 15"),
		AppBuild:   ptr("100"),
		AppVersion: ptr("1.0.0"),
	})
	require.NoError(t, err)
	require.Equal(t, deviceUUID, upsertedID)

	devices, err = db.ListDevicesByUser(ctx, projectID, userID)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, "device-revive", devices[0].DeviceID)
	require.NotNil(t, devices[0].Token)
	require.Equal(t, "token-revive-new", *devices[0].Token)
	require.NotNil(t, devices[0].OS)
	require.Equal(t, "ios", *devices[0].OS)
}
