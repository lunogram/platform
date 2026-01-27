package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/container"
	"github.com/stretchr/testify/require"
)

func TestS3StorageWrite(t *testing.T) {
	endpoint := container.RunLocalStack(t)
	ctx := context.Background()

	storage, err := NewS3Storage(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Endpoint:  endpoint,
		AccessKey: "test",
		SecretKey: "test",
	})
	require.NoError(t, err)

	// Create bucket
	_, err = storage.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("test-bucket"),
	})
	require.NoError(t, err)

	id := uuid.New()
	key := fmt.Sprintf("%s.jpg", id)
	content := []byte("fake jpeg for S3")
	reader := bytes.NewReader(content)

	err = storage.Write(ctx, key, reader)
	require.NoError(t, err)
}

func TestS3StorageRead(t *testing.T) {
	endpoint := container.RunLocalStack(t)
	ctx := context.Background()

	storage, err := NewS3Storage(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Endpoint:  endpoint,
		AccessKey: "test",
		SecretKey: "test",
	})
	require.NoError(t, err)

	// Create bucket
	_, err = storage.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("test-bucket"),
	})
	require.NoError(t, err)

	id := uuid.New()
	key := fmt.Sprintf("%s.png", id)
	content := []byte("fake png for reading")
	reader := bytes.NewReader(content)

	err = storage.Write(ctx, key, reader)
	require.NoError(t, err)

	readCloser, err := storage.Read(ctx, key)
	require.NoError(t, err)
	defer readCloser.Close()

	readContent, err := io.ReadAll(readCloser)
	require.NoError(t, err)
	require.Equal(t, content, readContent)
}

func TestS3StorageReadNotFound(t *testing.T) {
	endpoint := container.RunLocalStack(t)
	ctx := context.Background()

	storage, err := NewS3Storage(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Endpoint:  endpoint,
		AccessKey: "test",
		SecretKey: "test",
	})
	require.NoError(t, err)

	// Create bucket
	_, err = storage.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("test-bucket"),
	})
	require.NoError(t, err)

	id := uuid.New()
	key := fmt.Sprintf("%s.webp", id)

	_, err = storage.Read(ctx, key)
	require.Error(t, err)
}

func TestS3StorageDelete(t *testing.T) {
	endpoint := container.RunLocalStack(t)
	ctx := context.Background()

	storage, err := NewS3Storage(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Endpoint:  endpoint,
		AccessKey: "test",
		SecretKey: "test",
	})
	require.NoError(t, err)

	// Create bucket
	_, err = storage.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("test-bucket"),
	})
	require.NoError(t, err)

	id := uuid.New()
	key := fmt.Sprintf("%s.gif", id)
	content := []byte("fake gif for deletion")
	reader := bytes.NewReader(content)

	err = storage.Write(ctx, key, reader)
	require.NoError(t, err)

	err = storage.Delete(ctx, key)
	require.NoError(t, err)

	_, err = storage.Read(ctx, key)
	require.Error(t, err)
}

func TestS3StorageDeleteNotFound(t *testing.T) {
	endpoint := container.RunLocalStack(t)
	ctx := context.Background()

	storage, err := NewS3Storage(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Endpoint:  endpoint,
		AccessKey: "test",
		SecretKey: "test",
	})
	require.NoError(t, err)

	// Create bucket
	_, err = storage.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("test-bucket"),
	})
	require.NoError(t, err)

	id := uuid.New()
	key := fmt.Sprintf("%s.pdf", id)

	err = storage.Delete(ctx, key)
	require.NoError(t, err)
}

func TestS3StorageMultipleFiles(t *testing.T) {
	endpoint := container.RunLocalStack(t)
	ctx := context.Background()

	storage, err := NewS3Storage(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Endpoint:  endpoint,
		AccessKey: "test",
		SecretKey: "test",
	})
	require.NoError(t, err)

	// Create bucket
	_, err = storage.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("test-bucket"),
	})
	require.NoError(t, err)

	files := []struct {
		id      uuid.UUID
		ext     string
		key     string
		content []byte
	}{
		{uuid.New(), ".jpg", "", []byte("jpeg data")},
		{uuid.New(), ".png", "", []byte("png data")},
		{uuid.New(), ".mp4", "", []byte("video data")},
	}

	for i := range files {
		files[i].key = fmt.Sprintf("%s%s", files[i].id, files[i].ext)
		reader := bytes.NewReader(files[i].content)
		err := storage.Write(ctx, files[i].key, reader)
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
