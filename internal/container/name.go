package container

import "github.com/testcontainers/testcontainers-go"

// containerName scopes a reused container to the current test session.
// Every package within a single "go test" invocation shares one container, while
// independent runs — concurrent worktrees, a second run on the same machine — each
// get their own. A name shared across sessions is unsafe because reuse registers
// the container with the reusing session's reaper, so the first session to finish
// tears the container down while the others are still connected to it.
func containerName(kind string) string {
	id := testcontainers.SessionID()
	return "testcontainer-" + kind + "-" + id[:min(len(id), 12)]
}
