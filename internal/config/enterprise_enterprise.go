//go:build enterprise

package config

// Enterprise holds configuration that is only available in enterprise builds.
type Enterprise struct {
	Proxy EnterpriseProxy `envPrefix:"PROXY_" yaml:"proxy"`
}

// EnterpriseProxy holds upstream URLs for enterprise services. Only the
// services with a configured URL will have proxy routes registered.
type EnterpriseProxy struct {
	// BackofficeURL is the upstream URL for the backoffice service (e.g. http://backoffice).
	BackofficeURL string `env:"BACKOFFICE_URL" yaml:"backoffice_url"`
	// CourierURL is the upstream URL for the courier service (e.g. http://courier).
	CourierURL string `env:"COURIER_URL" yaml:"courier_url"`
}
