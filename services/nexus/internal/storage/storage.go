package storage

import (
	"context"
	"io"
)

type Config struct {
	Type          string      `env:"STORAGE_TYPE" envDefault:"local"`
	MaxUploadSize int64       `env:"STORAGE_MAX_UPLOAD_SIZE" envDefault:"10485760"` // 10MB default
	S3            S3Config    `envPrefix:"STORAGE_S3_"`
	Local         LocalConfig `envPrefix:"STORAGE_LOCAL_"`
}

type Storage interface {
	Write(ctx context.Context, key string, reader io.Reader) error
	Read(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

func New(cfg Config) (Storage, error) {
	switch cfg.Type {
	case "s3":
		return NewS3Storage(cfg.S3)
	default:
		return NewLocalStorage(cfg.Local)
	}
}
