package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/cluster/consensus"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/node/metrics"
	"go.uber.org/zap"
)

// CleanupTimeout defines the maximum duration for cleanup operations
// when a node is shutting down.
const CleanupTimeout = 5 * time.Second

type LeaderHandler func(ctx context.Context) error

func NewNode(ctx graceful.Context, logger *zap.Logger, conf config.Node, cluster *consensus.Cluster, handler LeaderHandler) (*Node, error) {
	id := conf.NodeID
	if id == "" {
		logger.Info("node ID not set, generating from hostname")
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}

		hash := sha256.Sum256([]byte(hostname))
		id = hex.EncodeToString(hash[:8])
	}

	node := &Node{
		id:            id,
		config:        conf,
		logger:        logger,
		cluster:       cluster,
		leaderHandler: handler,
	}

	node.wg.Add(3)
	go node.campaign(ctx)
	go node.heartbeat(ctx)
	go node.watchCluster(ctx)

	ctx.Closer(func() {
		node.wg.Wait()
	})

	return node, nil
}

type Node struct {
	id            string
	config        config.Node
	logger        *zap.Logger
	cluster       *consensus.Cluster
	leaderUntil   time.Time
	leaderHandler LeaderHandler
	mu            sync.Mutex
	wg            sync.WaitGroup
}

func (node *Node) ID() string {
	return node.id
}

func (node *Node) IsLeader() bool {
	node.mu.Lock()
	defer node.mu.Unlock()
	return node.leaderUntil.After(time.Now())
}

// go-campaign! starts campaigning for leadership in the cluster.
func (node *Node) campaign(ctx graceful.Context) {
	defer node.wg.Done()
	defer func() {
		if node.IsLeader() {
			ctx, cancel := context.WithTimeout(context.Background(), CleanupTimeout)
			defer cancel()

			node.cluster.ReleaseLeader(ctx)
		}
	}()

	var err error
	leaderCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if node.IsLeader() {
			until := node.cluster.Extend(ctx)
			node.mu.Lock()
			node.leaderUntil = until
			node.mu.Unlock()
			node.logger.Debug("extending leadership", zap.Time("until", until))

			if !until.After(time.Now()) {
				node.logger.Error("failed to extend leadership")
				metrics.LeaderElectionFailuresTotal.Inc()
			}

			err = node.cluster.RegisterLeader(ctx, node.id)
			if err != nil {
				node.logger.Error("failed to register leader", zap.Error(err))
			}

			// NOTE: we do not want to spam the cluster with campaign requests,
			time.Sleep(node.config.Cluster.LeaderCampaignInterval)
			continue
		}

		until := node.cluster.Lock(ctx)
		node.mu.Lock()
		node.leaderUntil = until
		node.mu.Unlock()

		if !node.IsLeader() {
			// NOTE: we have to cancel the leader context if we are not elected
			// ensuring that the leader routine is stopped.
			if leaderCtx.Err() == nil {
				cancel()
			}

			time.Sleep(nodeCampaignInterval())
			continue
		}

		node.logger.Info("node is elected as leader")
		metrics.LeaderElectionsTotal.Inc()
		err = node.cluster.RegisterLeader(ctx, node.id)
		if err != nil {
			node.logger.Error("failed to register leader", zap.Error(err))
		}

		leaderCtx, cancel = context.WithCancel(ctx)

		go func() {
			defer cancel()
			err := node.leaderHandler(leaderCtx)
			if err != nil {
				node.logger.Error("unexpected error in leader routine", zap.Error(err))
			}

			node.logger.Info("releasing leader lock", zap.String("node", node.id))
			ctx, cancel := context.WithTimeout(context.Background(), CleanupTimeout)
			defer cancel()

			until := node.cluster.ReleaseLeader(ctx)

			node.mu.Lock()
			node.leaderUntil = until
			node.mu.Unlock()
		}()

		time.Sleep(node.config.Cluster.LeaderCampaignInterval)
	}
}

// watchCluster watches for changes in the cluster and updates the node's
// internal state accordingly. It listens for node updates and reconciles the
// cluster state periodically.
func (node *Node) watchCluster(ctx graceful.Context) {
	defer node.wg.Done()

	pubsub := node.cluster.WatchNodes(ctx)
	reconciliation := time.NewTicker(node.config.Cluster.ReconciliationInterval)
	defer reconciliation.Stop()

	err := node.cluster.MarkLeaderReconciled(ctx)
	if err != nil {
		node.logger.Error("failed to mark leader as reconciled", zap.Error(err))
		go ctx.Shutdown()
		return
	}

	var nodes map[string]string

	for {
		select {
		case <-ctx.Done():
			return
		case <-reconciliation.C:
			nodes, err = node.cluster.GetNodes(ctx)
			if err != nil {
				node.logger.Error("failed to fetch nodes", zap.Error(err))
				go ctx.Shutdown()
				return
			}
		case nodes = <-pubsub:
		}

		metrics.TotalNodes.Set(float64(len(nodes)))
	}
}

func (node *Node) heartbeat(ctx graceful.Context) {
	defer node.wg.Done()
	defer func() {
		node.mu.Lock()
		defer node.mu.Unlock()

		logger := node.logger.With(zap.String("id", node.ID()))
		logger.Info("received close signal, releasing node resources")

		ctx, cancel := context.WithTimeout(context.Background(), CleanupTimeout)
		defer cancel()
		err := node.cluster.ReleaseNode(ctx, node.ID())
		if err != nil {
			logger.Error("failed to release node", zap.Error(err))
		}

		logger.Info("node resources released")
	}()

	heartbeat := time.NewTicker(node.config.Cluster.HeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			node.logger.Debug("heartbeat", zap.String("node", node.id))
			err := node.cluster.RegisterNode(ctx, node.id, node.id)
			if err != nil {
				node.logger.Info("failed to register node", zap.Error(err), zap.String("node", node.id))
			}
		}
	}
}

// nodeCampaignInterval returns a random campaign interval for the node
// between 1 and 3 seconds.
func nodeCampaignInterval() time.Duration {
	return time.Duration(1000+rand.Intn(2000)) * time.Millisecond
}
