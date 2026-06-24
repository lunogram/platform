package rbac

type Config struct {
	PostgresURI string `env:"POSTGRES_URI" envDefault:"postgres://postgres:postgrespw@postgres:5432/openfga?sslmode=disable"`
}
