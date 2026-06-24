package v1

import "github.com/lunogram/platform/internal/store/management"

var DefaultProject = management.Project{
	Name:     "Test Project",
	Timezone: "UTC",
	Locale:   "en",
}
