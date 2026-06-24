//go:build !enterprise

package config

// Enterprise is empty in OSS builds. Enterprise configuration
// is only available in enterprise builds.
type Enterprise struct{}
