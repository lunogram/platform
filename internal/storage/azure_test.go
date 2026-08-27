package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/stretchr/testify/require"
)

// newAzuriteStorage boots Azurite and returns storage bound to a freshly
// created blob container. The Azurite container is reused across runs, so each
// test takes a uniquely named blob container rather than sharing one and
// inheriting blobs an earlier run left behind.
func newAzuriteStorage(t *testing.T) *AzureStorage {
	t.Helper()

	endpoint := container.RunAzurite(t)
	name := fmt.Sprintf("test-%s", uuid.New())

	storage, err := NewAzureStorage(AzureConfig{
		Account:    container.AzuriteAccount,
		AccountKey: container.AzuriteAccountKey,
		Container:  name,
		Endpoint:   endpoint,
	})
	require.NoError(t, err)

	_, err = storage.client.CreateContainer(context.Background(), name, nil)
	require.NoError(t, err)

	return storage
}

func TestAzureStorageWrite(t *testing.T) {
	storage := newAzuriteStorage(t)
	ctx := context.Background()

	key := fmt.Sprintf("%s.jpg", uuid.New())
	content := []byte("fake jpeg for Azure")

	err := storage.Write(ctx, key, bytes.NewReader(content), "image/jpeg")
	require.NoError(t, err)
}

func TestAzureStorageRead(t *testing.T) {
	storage := newAzuriteStorage(t)
	ctx := context.Background()

	key := fmt.Sprintf("%s.png", uuid.New())
	content := []byte("fake png for reading")

	err := storage.Write(ctx, key, bytes.NewReader(content), "image/png")
	require.NoError(t, err)

	readCloser, err := storage.Read(ctx, key)
	require.NoError(t, err)
	defer readCloser.Close()

	readContent, err := io.ReadAll(readCloser)
	require.NoError(t, err)
	require.Equal(t, content, readContent)
}

func TestAzureStorageReadNotFound(t *testing.T) {
	storage := newAzuriteStorage(t)
	ctx := context.Background()

	key := fmt.Sprintf("%s.webp", uuid.New())

	_, err := storage.Read(ctx, key)
	require.Error(t, err)
}

func TestAzureStorageDelete(t *testing.T) {
	storage := newAzuriteStorage(t)
	ctx := context.Background()

	key := fmt.Sprintf("%s.gif", uuid.New())
	content := []byte("fake gif for deletion")

	err := storage.Write(ctx, key, bytes.NewReader(content), "image/gif")
	require.NoError(t, err)

	err = storage.Delete(ctx, key)
	require.NoError(t, err)

	_, err = storage.Read(ctx, key)
	require.Error(t, err)
}

// TestAzureStorageDeleteNotFound pins the same contract the S3 and local
// drivers keep: deleting a key that is not there is not an error. Azure
// reports BlobNotFound where the others stay silent.
func TestAzureStorageDeleteNotFound(t *testing.T) {
	storage := newAzuriteStorage(t)
	ctx := context.Background()

	key := fmt.Sprintf("%s.pdf", uuid.New())

	err := storage.Delete(ctx, key)
	require.NoError(t, err)
}

func TestAzureStorageWriteRecordsContentType(t *testing.T) {
	storage := newAzuriteStorage(t)
	ctx := context.Background()

	key := fmt.Sprintf("%s.png", uuid.New())

	err := storage.Write(ctx, key, bytes.NewReader([]byte("png data")), "image/png")
	require.NoError(t, err)

	blobClient := storage.client.ServiceClient().NewContainerClient(storage.container).NewBlobClient(key)
	properties, err := blobClient.GetProperties(ctx, &blob.GetPropertiesOptions{})
	require.NoError(t, err)
	require.NotNil(t, properties.ContentType)
	require.Equal(t, "image/png", *properties.ContentType)
}

func TestAzureStorageMultipleFiles(t *testing.T) {
	storage := newAzuriteStorage(t)
	ctx := context.Background()

	files := []struct {
		key         string
		contentType string
		content     []byte
	}{
		{fmt.Sprintf("%s.jpg", uuid.New()), "image/jpeg", []byte("jpeg data")},
		{fmt.Sprintf("%s.png", uuid.New()), "image/png", []byte("png data")},
		{fmt.Sprintf("%s.mp4", uuid.New()), "video/mp4", []byte("video data")},
	}

	for _, f := range files {
		err := storage.Write(ctx, f.key, bytes.NewReader(f.content), f.contentType)
		require.NoError(t, err)
	}

	for _, f := range files {
		readCloser, err := storage.Read(ctx, f.key)
		require.NoError(t, err)

		readContent, err := io.ReadAll(readCloser)
		require.NoError(t, err)
		require.Equal(t, f.content, readContent)

		readCloser.Close()
	}

	for _, f := range files {
		err := storage.Delete(ctx, f.key)
		require.NoError(t, err)

		_, err = storage.Read(ctx, f.key)
		require.Error(t, err)
	}
}

func TestAzureConfigServiceURL(t *testing.T) {
	require.Equal(t, "https://acme.blob.core.windows.net", AzureConfig{Account: "acme"}.ServiceURL())
	require.Equal(t, "http://127.0.0.1:10000/devstoreaccount1", AzureConfig{
		Account:  "devstoreaccount1",
		Endpoint: "http://127.0.0.1:10000/devstoreaccount1/",
	}.ServiceURL())
}

func TestNewAzureStorageRequiresConfig(t *testing.T) {
	_, err := NewAzureStorage(AzureConfig{Container: "media", AccountKey: "key"})
	require.ErrorContains(t, err, "account is required")

	_, err = NewAzureStorage(AzureConfig{Account: "acme", AccountKey: "key"})
	require.ErrorContains(t, err, "container is required")

	_, err = NewAzureStorage(AzureConfig{Account: "acme", Container: "media"})
	require.ErrorContains(t, err, "account key is required")
}
