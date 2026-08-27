package container

import (
	"strings"
	"testing"

	_ "embed"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

//go:embed redis.conf
var redisConfig string

func RunRedis(t *testing.T) string {
	t.Helper()

	container, err := redis.Run(t.Context(),
		"redis:8",
		redis.WithSnapshotting(10, 1),
		testcontainers.WithLogger(log.TestLogger(t)),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			Reader:            strings.NewReader(redisConfig),
			ContainerFilePath: "/usr/local/redis.conf",
			FileMode:          0o755,
		}),
		testcontainers.WithReuseByName(containerName("redis")),
	)
	require.NoError(t, err)

	connstr, err := container.ConnectionString(t.Context())
	require.NoError(t, err)

	return connstr
}
