package storage

import (
	"context"
	"io"
	"strings"
)

type Config struct {
	Type          string      `env:"STORAGE_TYPE" envDefault:"local"`
	BaseURL       string      `env:"STORAGE_BASE_URL"`
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

// URLResolver generates public URLs for stored documents.
// When a BaseURL is configured (e.g. a CDN or S3 bucket URL), it constructs
// URLs as {BaseURL}/{key}. Otherwise it falls back to the application's
// public URL with a local serving path.
type URLResolver struct {
	baseURL string
}

// NewURLResolver creates a URLResolver. If storageBaseURL is set it takes
// precedence. Otherwise the publicURL of the application is used with
// the /uploads/documents/ path as fallback for local storage.
func NewURLResolver(storageBaseURL, publicURL string) *URLResolver {
	base := storageBaseURL
	if base == "" {
		base = strings.TrimRight(publicURL, "/") + "/uploads/documents"
	}
	base = strings.TrimRight(base, "/")
	return &URLResolver{baseURL: base}
}

// URL returns the public URL for the given storage key.
func (r *URLResolver) URL(key string) string {
	return r.baseURL + "/" + key
}
