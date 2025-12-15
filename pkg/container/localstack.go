package container

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
)

// RunLocalStack runs a LocalStack container for testing and returns the endpoint URL.
// LocalStack provides local AWS cloud stack for development and testing.
func RunLocalStack(t *testing.T) (endpoint string) {
	t.Helper()
	ctx := context.Background()

	container, err := localstack.Run(ctx,
		"localstack/localstack:4.11",
		testcontainers.WithReuseByName("localstack"),
	)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, nat.Port("4566/tcp"))
	require.NoError(t, err)

	provider, err := testcontainers.NewDockerProvider()
	require.NoError(t, err)
	defer provider.Close()

	host, err := provider.DaemonHost(ctx)
	require.NoError(t, err)

	return fmt.Sprintf("http://%s:%d", host, mappedPort.Int())
}
