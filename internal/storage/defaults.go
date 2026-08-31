package storage

// DefaultConfig returns the storage settings used when nothing configures them.
func DefaultConfig() Config {
	return Config{
		Type:          "local",
		MaxUploadSize: 10485760,
		Local:         LocalConfig{Directory: "./uploads/documents"},
	}
}
