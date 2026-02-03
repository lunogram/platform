package consensus

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newCluster(t *testing.T) (*Cluster, graceful.Context) {
	t.Helper()
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	connstr := container.RunRedis(t)
	conf := config.Node{
		Redis: config.Redis{
			Address:   connstr,
			KeyPrefix: fmt.Sprintf("%s:", uuid.New()),
		},
	}
	cluster, err := NewCluster(ctx, logger, conf)
	require.NoError(t, err)
	require.NotNil(t, cluster)
	return cluster, ctx
}

func TestNewCluster(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())

	type test struct {
		address     string
		expectError bool
	}

	tests := map[string]test{
		"valid redis URL": {
			address:     container.RunRedis(t),
			expectError: false,
		},
		"invalid redis URL": {
			address:     "invalid://url",
			expectError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			conf := config.Node{
				Redis: config.Redis{
					Address:   test.address,
					KeyPrefix: fmt.Sprintf("%s:", uuid.New()),
				},
			}
			cluster, err := NewCluster(ctx, logger, conf)
			if test.expectError {
				require.Error(t, err)
				require.Nil(t, cluster)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cluster)
				require.NotNil(t, cluster.redis)
				require.NotNil(t, cluster.mu)
			}
		})
	}
}

func TestClusterRegisterLeader(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	err := cluster.RegisterLeader(ctx, "test-leader-1")
	require.NoError(t, err)

	leaderID, err := cluster.redis.Get(ctx, cluster.key(LeaderKey)).Result()
	require.NoError(t, err)
	require.Equal(t, "test-leader-1", leaderID)

	ttl, err := cluster.redis.TTL(ctx, cluster.key(LeaderKey)).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, DefaultLeaderTTL)
}

func TestClusterMarkLeaderReconciled(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	err := cluster.MarkLeaderReconciled(ctx)
	require.NoError(t, err)

	result, err := cluster.redis.Get(ctx, cluster.key(LeaderKeyReconciled)).Bool()
	require.NoError(t, err)
	require.Equal(t, true, result)

	ttl, err := cluster.redis.TTL(ctx, cluster.key(LeaderKeyReconciled)).Result()
	require.NoError(t, err)
	require.Equal(t, time.Duration(-1), ttl)
}

func TestClusterReleaseLeader(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	err := cluster.RegisterLeader(ctx, "test-leader")
	require.NoError(t, err)

	err = cluster.MarkLeaderReconciled(ctx)
	require.NoError(t, err)

	until := cluster.ReleaseLeader(ctx)
	require.True(t, until.IsZero())

	_, err = cluster.redis.Get(ctx, cluster.key(LeaderKey)).Result()
	require.Error(t, err)

	_, err = cluster.redis.Get(ctx, cluster.key(LeaderKeyReconciled)).Result()
	require.Error(t, err)
}

func TestClusterRegisterNode(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	type test struct {
		nodeID        string
		address       string
		expectError   bool
		shouldPublish bool
	}

	tests := map[string]test{
		"first registration": {
			nodeID:        "node-1",
			address:       "127.0.0.1:8080",
			expectError:   false,
			shouldPublish: true,
		},
		"re-registration": {
			nodeID:        "node-1",
			address:       "127.0.0.1:8080",
			expectError:   false,
			shouldPublish: false,
		},
		"different node": {
			nodeID:        "node-2",
			address:       "127.0.0.1:8081",
			expectError:   false,
			shouldPublish: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := cluster.RegisterNode(ctx, test.nodeID, test.address)
			if test.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			nodeKey := cluster.key("cluster:node:" + test.nodeID)
			address, err := cluster.redis.Get(ctx, nodeKey).Result()
			require.NoError(t, err)
			require.Equal(t, test.address, address)

			ttl, err := cluster.redis.TTL(ctx, nodeKey).Result()
			require.NoError(t, err)
			require.Greater(t, ttl, time.Duration(0))
			require.LessOrEqual(t, ttl, DefaultNodeTTL)
		})
	}
}

func TestClusterReleaseNode(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	err := cluster.RegisterNode(ctx, "test-node", "127.0.0.1:8080")
	require.NoError(t, err)

	nodeKey := cluster.key("cluster:node:test-node")
	_, err = cluster.redis.Get(ctx, nodeKey).Result()
	require.NoError(t, err)

	err = cluster.ReleaseNode(ctx, "test-node")
	require.NoError(t, err)

	_, err = cluster.redis.Get(ctx, nodeKey).Result()
	require.Error(t, err)
}

func TestClusterGetNodes(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	// Test no nodes
	nodes, err := cluster.GetNodes(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]string{}, nodes)

	// Test single node
	err = cluster.RegisterNode(ctx, "node-1", "127.0.0.1:8080")
	require.NoError(t, err)
	nodes, err = cluster.GetNodes(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"node-1": "127.0.0.1:8080",
	}, nodes)

	// Test multiple nodes
	err = cluster.RegisterNode(ctx, "node-2", "127.0.0.1:8081")
	require.NoError(t, err)
	err = cluster.RegisterNode(ctx, "node-3", "127.0.0.1:8082")
	require.NoError(t, err)
	nodes, err = cluster.GetNodes(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"node-1": "127.0.0.1:8080",
		"node-2": "127.0.0.1:8081",
		"node-3": "127.0.0.1:8082",
	}, nodes)
}

func TestClusterWatchNodes(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	nodesChan := cluster.WatchNodes(ctx)

	err := cluster.RegisterNode(ctx, "node-1", "127.0.0.1:8080")
	require.NoError(t, err)

	select {
	case nodes := <-nodesChan:
		require.Contains(t, nodes, "node-1")
		require.Equal(t, "127.0.0.1:8080", nodes["node-1"])
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for node registration event")
	}

	err = cluster.RegisterNode(ctx, "node-2", "127.0.0.1:8081")
	require.NoError(t, err)

	select {
	case nodes := <-nodesChan:
		require.Contains(t, nodes, "node-1")
		require.Contains(t, nodes, "node-2")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for second node registration event")
	}

	err = cluster.ReleaseNode(ctx, "node-1")
	require.NoError(t, err)

	select {
	case nodes := <-nodesChan:
		require.NotContains(t, nodes, "node-1")
		require.Contains(t, nodes, "node-2")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for node release event")
	}
}

func TestClusterLock(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	until := cluster.Lock(ctx)
	require.False(t, until.IsZero())
	require.True(t, until.After(time.Now()))

	until2 := cluster.Lock(ctx)
	require.False(t, until2.IsZero())
	require.True(t, until.After(until2) || until.Equal(until2))
}

func TestClusterExtend(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	until := cluster.Lock(ctx)
	require.False(t, until.IsZero())

	time.Sleep(100 * time.Millisecond)

	extended := cluster.Extend(ctx)
	require.False(t, extended.IsZero())
	require.True(t, extended.After(until) || extended.Equal(until))
}

func TestClusterUnlock(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	until := cluster.Lock(ctx)
	require.False(t, until.IsZero())

	err := cluster.Unlock(ctx)
	require.NoError(t, err)

	err = cluster.Unlock(ctx)
	require.NoError(t, err)
}

func TestClusterLeaderMutex(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	connstr := container.RunRedis(t)

	// Create two clusters sharing the same Redis instance and prefix
	conf := config.Node{
		Redis: config.Redis{
			Address:   connstr,
			KeyPrefix: fmt.Sprintf("%s:", uuid.New()),
		},
	}
	cluster1, err := NewCluster(ctx, logger, conf)
	require.NoError(t, err)

	cluster2, err := NewCluster(ctx, logger, conf)
	require.NoError(t, err)

	until1 := cluster1.Lock(ctx)
	require.False(t, until1.IsZero())
	require.True(t, until1.After(time.Now()))

	// cluster2 tries to lock but cluster1 already has it
	// TryLock fails, returns zero time
	until2 := cluster2.Lock(ctx)
	require.True(t, until2.IsZero() || until2.Before(time.Now()) || until2.Equal(until1))

	err = cluster1.Unlock(ctx)
	require.NoError(t, err)

	// Now cluster2 should be able to acquire the lock
	until3 := cluster2.Lock(ctx)
	require.False(t, until3.IsZero())
	require.True(t, until3.After(time.Now()))
}

func TestClusterNodeTTL(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	err := cluster.RegisterNode(ctx, "ttl-node", "127.0.0.1:8080")
	require.NoError(t, err)

	nodes, err := cluster.GetNodes(ctx)
	require.NoError(t, err)
	require.Contains(t, nodes, "ttl-node")

	time.Sleep(DefaultNodeTTL + time.Second)

	nodes, err = cluster.GetNodes(ctx)
	require.NoError(t, err)
	require.NotContains(t, nodes, "ttl-node", "node should have expired after TTL")
}

func TestClusterLeaderTTL(t *testing.T) {
	t.Parallel()
	cluster, ctx := newCluster(t)

	err := cluster.RegisterLeader(ctx, "ttl-leader")
	require.NoError(t, err)

	leaderID, err := cluster.redis.Get(ctx, cluster.key(LeaderKey)).Result()
	require.NoError(t, err)
	require.Equal(t, "ttl-leader", leaderID)

	time.Sleep(DefaultLeaderTTL + time.Second)

	_, err = cluster.redis.Get(ctx, cluster.key(LeaderKey)).Result()
	require.Error(t, err, "leader key should have expired after TTL")
}
