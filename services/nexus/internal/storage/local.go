package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalConfig struct {
	Directory string `env:"UPLOAD_DIRECTORY" envDefault:"./uploads/images"`
}

func NewLocalStorage(cnf LocalConfig) (*LocalStorage, error) {
	if err := os.MkdirAll(cnf.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		directory: cnf.Directory,
	}, nil
}

type LocalStorage struct {
	directory string
}

func (l *LocalStorage) Write(ctx context.Context, key string, reader io.Reader) error {
	filePath := filepath.Join(l.directory, key)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (l *LocalStorage) Read(ctx context.Context, key string) (io.ReadCloser, error) {
	filePath := filepath.Join(l.directory, key)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

func (l *LocalStorage) Delete(ctx context.Context, key string) error {
	filePath := filepath.Join(l.directory, key)

	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
