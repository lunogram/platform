package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLocalStorageWrite(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{Directory: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	id := uuid.New()
	key := fmt.Sprintf("%s.jpg", id)
	content := []byte("fake jpeg content")

	err = storage.Write(ctx, key, bytes.NewReader(content))
	require.NoError(t, err)

	// Verify file was created on disk
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func TestLocalStorageRead(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{Directory: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	id := uuid.New()
	key := fmt.Sprintf("%s.png", id)
	content := []byte("fake png content")

	// Write file first
	err = storage.Write(ctx, key, bytes.NewReader(content))
	require.NoError(t, err)

	// Read it back
	reader, err := storage.Read(ctx, key)
	require.NoError(t, err)
	defer reader.Close()

	readContent, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, content, readContent)
}

func TestLocalStorageReadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{Directory: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	id := uuid.New()
	key := fmt.Sprintf("%s.webp", id)

	_, err = storage.Read(ctx, key)
	require.Error(t, err)
}

func TestLocalStorageDelete(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{Directory: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	id := uuid.New()
	key := fmt.Sprintf("%s.gif", id)
	content := []byte("fake gif content")

	// Write file first
	err = storage.Write(ctx, key, bytes.NewReader(content))
	require.NoError(t, err)

	// Delete it
	err = storage.Delete(ctx, key)
	require.NoError(t, err)

	// Verify file was deleted
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, files, 0)
}

func TestLocalStorageDeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{Directory: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	id := uuid.New()
	key := fmt.Sprintf("%s.pdf", id)

	// Should not error when deleting non-existent file
	err = storage.Delete(ctx, key)
	require.NoError(t, err)
}

func TestLocalStorageMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{Directory: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	// Write multiple files
	files := []struct {
		id      uuid.UUID
		ext     string
		key     string
		content []byte
	}{
		{uuid.New(), ".jpg", "", []byte("jpeg content")},
		{uuid.New(), ".png", "", []byte("png content")},
		{uuid.New(), ".mp4", "", []byte("video content")},
	}

	for i := range files {
		files[i].key = fmt.Sprintf("%s%s", files[i].id, files[i].ext)
		err := storage.Write(ctx, files[i].key, bytes.NewReader(files[i].content))
		require.NoError(t, err)
	}

	// Read them back
	for _, f := range files {
		reader, err := storage.Read(ctx, f.key)
		require.NoError(t, err)

		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, f.content, content)
		reader.Close()
	}

	// Delete one
	err = storage.Delete(ctx, files[1].key)
	require.NoError(t, err)

	// Verify others still exist
	_, err = storage.Read(ctx, files[0].key)
	require.NoError(t, err)

	_, err = storage.Read(ctx, files[2].key)
	require.NoError(t, err)

	// Deleted one should not exist
	_, err = storage.Read(ctx, files[1].key)
	require.Error(t, err)
}
