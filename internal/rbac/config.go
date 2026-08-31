package rbac

type Config struct {
	PostgresURI string `env:"POSTGRES_URI" yaml:"postgres_uri"`
}
