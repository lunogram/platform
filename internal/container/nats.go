package container

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
)

func RunNATS(t *testing.T) string {
	t.Helper()

	container, err := nats.Run(t.Context(), "nats:2.9",
		testcontainers.WithReuseByName("testcontainer-nats"),
	)
	require.NoError(t, err)

	connstr, err := container.ConnectionString(t.Context())
	require.NoError(t, err)

	return connstr
}
