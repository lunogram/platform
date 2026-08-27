package storage

import (
	"context"
	"io"
)

type Config struct {
	Type          string      `env:"STORAGE_TYPE" envDefault:"local"`
	BaseURL       string      `env:"STORAGE_BASE_URL"`
	MaxUploadSize int64       `env:"STORAGE_MAX_UPLOAD_SIZE" envDefault:"10485760"` // 10MB default
	S3            S3Config    `envPrefix:"STORAGE_S3_"`
	Azure         AzureConfig `envPrefix:"STORAGE_AZURE_"`
	Local         LocalConfig `envPrefix:"STORAGE_LOCAL_"`
}

type Storage interface {
	// Write stores the reader's contents under key. The contentType is
	// recorded as object metadata where the backend supports it, so that
	// media served straight from the bucket over STORAGE_BASE_URL carries a
	// usable Content-Type rather than a generic binary fallback.
	Write(ctx context.Context, key string, reader io.Reader, contentType string) error
	Read(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

func New(cfg Config) (Storage, error) {
	switch cfg.Type {
	case "s3":
		return NewS3Storage(cfg.S3)
	case "azure":
		return NewAzureStorage(cfg.Azure)
	default:
		return NewLocalStorage(cfg.Local)
	}
}

// URLResolver generates public URLs for stored documents.
// It constructs URLs as {baseURL}/{key} using the configured STORAGE_BASE_URL.
type URLResolver struct {
	baseURL string
}

// NewURLResolver creates a URLResolver from the configured storage base URL
// (e.g. a CDN or S3 bucket public URL).
func NewURLResolver(storageBaseURL string) *URLResolver {
	return &URLResolver{baseURL: storageBaseURL}
}

// URL returns the public URL for the given storage key.
func (r *URLResolver) URL(key string) string {
	return r.baseURL + "/" + key
}
