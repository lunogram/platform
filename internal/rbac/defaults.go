package rbac

// DefaultConfig returns the settings used when nothing configures them.
//
// Defaults live here rather than in an envDefault struct tag because
// configuration is layered: defaults first, then the YAML file, then the
// environment. A tag-supplied default is applied by env.Parse whenever the
// variable is unset, which would overwrite the YAML layer instead of yielding
// to it.
func DefaultConfig() Config {
	return Config{
		PostgresURI: "postgres://postgres:postgrespw@postgres:5432/openfga?sslmode=disable",
	}
}
