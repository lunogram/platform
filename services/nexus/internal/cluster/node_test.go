package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/cluster/consensus"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TNewCluster(t *testing.T) (*consensus.Cluster, graceful.Context) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	connstr := container.RunRedis(t)
	conf := config.Node{
		Redis: config.Redis{
			Address:   connstr,
			KeyPrefix: fmt.Sprintf("%s:", uuid.New()),
		},
		Cluster: config.Cluster{
			HeartbeatInterval:      50 * time.Millisecond,
			LeaderCampaignInterval: 50 * time.Millisecond,
			ReconciliationInterval: 100 * time.Millisecond,
		},
	}

	cluster, err := consensus.NewCluster(ctx, logger, conf)
	require.NoError(t, err)

	return cluster, ctx
}

func TestNewNode(t *testing.T) {
	t.Parallel()

	type test struct {
		config        config.Node
		expectError   bool
		checkNodeID   bool
		expectedIDLen int
	}

	tests := map[string]test{
		"with explicit node ID": {
			config: config.Node{
				NodeID: "test-node-123",
				Cluster: config.Cluster{
					HeartbeatInterval:      100 * time.Millisecond,
					LeaderCampaignInterval: 100 * time.Millisecond,
					ReconciliationInterval: 100 * time.Millisecond,
				},
			},
			expectError:   false,
			checkNodeID:   true,
			expectedIDLen: 13,
		},
		"without node ID generates from hostname": {
			config: config.Node{
				NodeID: "",
				Cluster: config.Cluster{
					HeartbeatInterval:      100 * time.Millisecond,
					LeaderCampaignInterval: 100 * time.Millisecond,
					ReconciliationInterval: 100 * time.Millisecond,
				},
			},
			expectError:   false,
			checkNodeID:   true,
			expectedIDLen: 16,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cluster, ctx := TNewCluster(t)

			handler := func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			}

			node, err := NewNode(ctx, logger, test.config, cluster, handler)

			if test.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, node)

			if test.checkNodeID {
				require.NotEmpty(t, node.ID())
				if test.expectedIDLen > 0 {
					require.Len(t, node.ID(), test.expectedIDLen)
				}
			}

			time.Sleep(200 * time.Millisecond)
			ctx.Shutdown()
		})
	}
}

func TestNodeID(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	cluster, ctx := TNewCluster(t)

	conf := config.Node{
		NodeID: "custom-node-id",
		Cluster: config.Cluster{
			HeartbeatInterval:      100 * time.Millisecond,
			LeaderCampaignInterval: 100 * time.Millisecond,
			ReconciliationInterval: 100 * time.Millisecond,
		},
	}

	handler := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	node, err := NewNode(ctx, logger, conf, cluster, handler)
	require.NoError(t, err)

	require.Equal(t, "custom-node-id", node.ID())

	time.Sleep(50 * time.Millisecond)
	ctx.Shutdown()
}

func TestNodeIsLeader(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	cluster, ctx := TNewCluster(t)

	conf := config.Node{
		NodeID: "test-leader-node",
		Cluster: config.Cluster{
			HeartbeatInterval:      50 * time.Millisecond,
			LeaderCampaignInterval: 50 * time.Millisecond,
			ReconciliationInterval: 50 * time.Millisecond,
		},
	}

	handler := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	node, err := NewNode(ctx, logger, conf, cluster, handler)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return node.IsLeader()
	}, 2*time.Second, 50*time.Millisecond, "node should become leader")

	ctx.Shutdown()
	time.Sleep(100 * time.Millisecond)

	require.False(t, node.IsLeader(), "node should not be leader after shutdown")
}

func TestNodeHeartbeat(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	cluster, ctx := TNewCluster(t)

	conf := config.Node{
		NodeID: "heartbeat-test-node",
		Cluster: config.Cluster{
			HeartbeatInterval:      50 * time.Millisecond,
			LeaderCampaignInterval: 100 * time.Millisecond,
			ReconciliationInterval: 100 * time.Millisecond,
		},
	}

	handler := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	node, err := NewNode(ctx, logger, conf, cluster, handler)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	nodes, err := cluster.GetNodes(t.Context())
	require.NoError(t, err)
	require.Contains(t, nodes, node.ID(), "node should be registered in cluster")

	ctx.Shutdown()
	time.Sleep(100 * time.Millisecond)
}

func TestNodeLeaderHandlerExecution(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	cluster, ctx := TNewCluster(t)

	conf := config.Node{
		NodeID: "handler-test-node",
		Cluster: config.Cluster{
			HeartbeatInterval:      50 * time.Millisecond,
			LeaderCampaignInterval: 50 * time.Millisecond,
			ReconciliationInterval: 100 * time.Millisecond,
		},
	}

	handlerExecuted := make(chan struct{})
	handler := func(ctx context.Context) error {
		close(handlerExecuted)
		<-ctx.Done()
		return nil
	}

	node, err := NewNode(ctx, logger, conf, cluster, handler)
	require.NoError(t, err)

	select {
	case <-handlerExecuted:
	case <-time.After(3 * time.Second):
		t.Fatal("leader handler was not executed")
	}

	require.True(t, node.IsLeader(), "node should be leader when handler executes")

	ctx.Shutdown()
	time.Sleep(100 * time.Millisecond)
}

func TestMultipleNodesLeaderElection(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cluster, ctx := TNewCluster(t)

	conf1 := config.Node{
		NodeID: "node-1",
		Cluster: config.Cluster{
			HeartbeatInterval:      100 * time.Millisecond,
			LeaderCampaignInterval: 200 * time.Millisecond,
			ReconciliationInterval: 200 * time.Millisecond,
		},
	}

	conf2 := config.Node{
		NodeID: "node-2",
		Cluster: config.Cluster{
			HeartbeatInterval:      100 * time.Millisecond,
			LeaderCampaignInterval: 200 * time.Millisecond,
			ReconciliationInterval: 200 * time.Millisecond,
		},
	}

	handler := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	node1, err := NewNode(ctx, logger, conf1, cluster, handler)
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	node2, err := NewNode(ctx, logger, conf2, cluster, handler)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	require.True(t, node1.IsLeader() || node2.IsLeader(), "at least one node should be leader")

	nodes, err := cluster.GetNodes(t.Context())
	require.NoError(t, err)
	require.Len(t, nodes, 2, "both nodes should be registered")

	ctx.Shutdown()
	time.Sleep(100 * time.Millisecond)
}

func TestNodeWatchCluster(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	cluster, ctx := TNewCluster(t)

	conf := config.Node{
		NodeID: "watch-test-node",
		Cluster: config.Cluster{
			HeartbeatInterval:      50 * time.Millisecond,
			LeaderCampaignInterval: 50 * time.Millisecond,
			ReconciliationInterval: 100 * time.Millisecond,
		},
	}

	handler := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	node, err := NewNode(ctx, logger, conf, cluster, handler)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	err = cluster.RegisterNode(ctx, "external-node", "external-address")
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	nodes, err := cluster.GetNodes(ctx)
	require.NoError(t, err)
	require.Contains(t, nodes, node.ID())
	require.Contains(t, nodes, "external-node")
}

func TestNodeCleanupOnShutdown(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	cluster, ctx := TNewCluster(t)

	conf := config.Node{
		NodeID: "cleanup-test-node",
		Cluster: config.Cluster{
			HeartbeatInterval:      50 * time.Millisecond,
			LeaderCampaignInterval: 50 * time.Millisecond,
			ReconciliationInterval: 100 * time.Millisecond,
		},
	}

	handler := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	node, err := NewNode(ctx, logger, conf, cluster, handler)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	nodes, err := cluster.GetNodes(ctx)
	require.NoError(t, err)
	require.Contains(t, nodes, node.ID())

	ctx.Shutdown()
	time.Sleep(200 * time.Millisecond)
}

func TestNodeLeaderExtension(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	cluster, ctx := TNewCluster(t)

	conf := config.Node{
		NodeID: "extend-leader-node",
		Cluster: config.Cluster{
			HeartbeatInterval:      50 * time.Millisecond,
			LeaderCampaignInterval: 100 * time.Millisecond,
			ReconciliationInterval: 100 * time.Millisecond,
		},
	}

	handler := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	node, err := NewNode(ctx, logger, conf, cluster, handler)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return node.IsLeader()
	}, 2*time.Second, 50*time.Millisecond)

	time.Sleep(300 * time.Millisecond)

	require.True(t, node.IsLeader(), "node should maintain leadership through extensions")

	ctx.Shutdown()
	time.Sleep(100 * time.Millisecond)
}

func TestNodeCampaignInterval(t *testing.T) {
	t.Parallel()

	type test struct {
		expectedMin time.Duration
		expectedMax time.Duration
	}

	tests := map[string]test{
		"random interval between 1-3 seconds": {
			expectedMin: 1 * time.Second,
			expectedMax: 3 * time.Second,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 100; i++ {
				interval := nodeCampaignInterval()
				require.GreaterOrEqual(t, interval, test.expectedMin)
				require.LessOrEqual(t, interval, test.expectedMax)
			}
		})
	}
}
