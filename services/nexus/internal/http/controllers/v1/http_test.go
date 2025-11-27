package v1

import "github.com/lunogram/platform/services/nexus/internal/store"

var DefaultProject = store.Project{
	Name:     "Test Project",
	Timezone: "UTC",
	Locale:   "en-US",
}
