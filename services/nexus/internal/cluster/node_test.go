package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/cluster/consensus"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/container"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newCluster(t *testing.T, ctx graceful.Context) *consensus.Cluster {
	t.Helper()

	logger := zaptest.NewLogger(t)
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

	return cluster
}

func TestNewNode(t *testing.T) {
	t.Parallel()

	type test struct {
		config config.Node
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
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			ctx := graceful.NewContext(context.Background())
			cluster := newCluster(t, ctx)

			handler := func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			}

			node, err := NewNode(ctx, logger, test.config, cluster, handler)
			require.NoError(t, err)
			require.NotNil(t, node)
			require.NotEmpty(t, node.ID())

			t.Cleanup(func() {
				ctx.Shutdown()
			})
		})
	}
}

func TestNodeID(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	cluster := newCluster(t, ctx)

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

	t.Cleanup(func() {
		ctx.Shutdown()
	})
}

func TestNodeIsLeader(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	cluster := newCluster(t, ctx)

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

	tick := 50 * time.Millisecond
	timeout := 2 * time.Second

	require.Eventually(t, func() bool {
		return node.IsLeader()
	}, timeout, tick, "node should become leader")

	t.Cleanup(func() {
		ctx.Shutdown()
	})
}

func TestNodeHeartbeat(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	cluster := newCluster(t, ctx)

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

	t.Cleanup(func() {
		ctx.Shutdown()
	})
}

func TestNodeLeaderHandlerExecution(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	cluster := newCluster(t, ctx)

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
	t.Cleanup(func() {
		ctx.Shutdown()
	})
}

func TestMultipleNodesLeaderElection(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	cluster := newCluster(t, ctx)

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

	t.Cleanup(func() {
		ctx.Shutdown()
	})
}

func TestNodeWatchCluster(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	cluster := newCluster(t, ctx)

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

	err = cluster.RegisterNode(t.Context(), "external-node", "external-node")
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	nodes, err := cluster.GetNodes(ctx)
	require.NoError(t, err)
	require.Contains(t, nodes, node.ID())
	require.Contains(t, nodes, "external-node")

	t.Cleanup(func() {
		ctx.Shutdown()
	})
}

func TestNodeCleanupOnShutdown(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	cluster := newCluster(t, ctx)

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

	nodes, err := cluster.GetNodes(t.Context())
	require.NoError(t, err)
	require.Contains(t, nodes, node.ID())

	t.Cleanup(func() {
		ctx.Shutdown()
	})
}

func TestNodeLeaderExtension(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(context.Background())
	cluster := newCluster(t, ctx)

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

	t.Cleanup(func() {
		ctx.Shutdown()
	})
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
