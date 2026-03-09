package v1

import "github.com/lunogram/platform/internal/store/management"

var DefaultProject = management.Project{
	Name:     "Test Project",
	Timezone: "UTC",
	Locale:   "en",
}

func ptr[T any](v T) *T {
	return &v
}
