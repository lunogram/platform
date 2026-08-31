package store

// DefaultConfig returns the database URIs used when nothing configures them.
// See [github.com/lunogram/platform/internal/rbac.DefaultConfig] for why these
// are not envDefault tags.
func DefaultConfig() Config {
	return Config{
		ManagementURI: "postgres://postgres:postgrespw@postgres:5432/management?sslmode=disable",
		SubjectsURI:   "postgres://postgres:postgrespw@postgres:5432/subjects?sslmode=disable",
		JourneyURI:    "postgres://postgres:postgrespw@postgres:5432/journey?sslmode=disable",
	}
}
