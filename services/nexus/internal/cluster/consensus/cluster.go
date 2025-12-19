package consensus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/node/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// LeaderClusterMutex is the mutex used to ensure that only one node can be the
// leader at a time.
// https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/
const LeaderClusterMutex = "cluster:leader:mutex"

// DefaultNodeTTL is the default time to live for a node inside the cluster. The
// node will be removed from the cluster if it does not send a heartbeat within
// the given time to live.
const DefaultNodeTTL = 10 * time.Second

// DefaultLeaderTTL is the default time to live for a leader inside the cluster.
// The leader will be removed from the cluster if it does not extend the lock
// within the given time to live.
const DefaultLeaderTTL = 10 * time.Second

// ClusterRegisterNodeChannel is the channel used to notify other nodes about
// a new node being registered inside the cluster.
const ClusterRegisterNodeChannel = "cluster:node:add"

// ClusterReleaseNodeChannel is the channel used to notify other nodes about
// a node being released from the cluster.
const ClusterReleaseNodeChannel = "cluster:node:rel"

// LeaderKey is the key used to store the current leader inside the cluster.
// The key is used to ensure that only one node can be the leader at a time.
// The key is used to store the ID of the leader node.
const LeaderKey = "cluster:leader"

// LeaderKeyReconciled is the key used to mark the leader as reconciled. The key
// is used to mark that the leader has reconciled the cluster state. This key is
// often checked to check if the cluster is ready for operations.
const LeaderKeyReconciled = "cluster:leader:reconciled"

func NewCluster(ctx graceful.Context, logger *zap.Logger, service config.Node) (*Cluster, error) {
	logger.Info("initializing Redis client")

	options, err := redis.ParseURL(service.Redis.Address)
	if err != nil {
		return nil, err
	}

	rclient := redis.NewClient(options)
	ctx.Closer(func() {
		logger.Info("received close signal, closing redis client")
		err := rclient.Close()
		if err != nil && !errors.Is(err, redis.ErrClosed) {
			logger.Error("failed to close redis client", zap.Error(err))
		}
	})

	pool := goredis.NewPool(rclient)
	rs := redsync.New(pool)

	// Use prefix from service config, default to empty string
	prefix := service.Redis.KeyPrefix

	cluster := &Cluster{
		redis:  rclient,
		prefix: prefix,
		mu: rs.NewMutex(prefix+LeaderClusterMutex,
			redsync.WithExpiry(DefaultLeaderTTL),
			redsync.WithTries(1),
		),
	}

	return cluster, nil
}

type Cluster struct {
	redis  *redis.Client
	mu     *redsync.Mutex
	prefix string
}

// key returns the Redis key with the cluster prefix applied
func (cluster *Cluster) key(suffix string) string {
	return cluster.prefix + suffix
}

func (cluster *Cluster) RegisterLeader(ctx context.Context, id string) error {
	metrics.ClusterLeader.Set(1)
	return cluster.redis.Set(ctx, cluster.key(LeaderKey), id, DefaultLeaderTTL).Err()
}

func (cluster *Cluster) MarkLeaderReconciled(ctx context.Context) error {
	return cluster.redis.Set(ctx, cluster.key(LeaderKeyReconciled), true, 0).Err()
}

func (cluster *Cluster) ReleaseLeader(ctx context.Context) time.Time {
	metrics.ClusterLeader.Set(0)
	cluster.redis.Del(ctx, cluster.key(LeaderKey))
	cluster.redis.Del(ctx, cluster.key(LeaderKeyReconciled))
	cluster.Unlock(ctx) // nolint:errcheck
	return time.Time{}
}

func (cluster *Cluster) RegisterNode(ctx context.Context, id, address string) error {
	nodeKey := cluster.key(fmt.Sprintf("cluster:node:%s", id))
	exists, err := cluster.redis.Exists(ctx, nodeKey).Result()
	if err != nil {
		return err
	}

	err = cluster.redis.Set(ctx, nodeKey, address, DefaultNodeTTL).Err()
	if err != nil {
		return err
	}

	// NOTE: we should only publish the event during the first registration
	if exists == 0 {
		return cluster.redis.Publish(ctx, cluster.key(ClusterRegisterNodeChannel), id).Err()
	}

	return nil
}

func (cluster *Cluster) ReleaseNode(ctx context.Context, id string) error {
	nodeKey := cluster.key(fmt.Sprintf("cluster:node:%s", id))
	err := cluster.redis.Del(ctx, nodeKey).Err()
	if err != nil {
		return err
	}

	return cluster.redis.Publish(ctx, cluster.key(ClusterReleaseNodeChannel), id).Err()
}

func (cluster *Cluster) WatchNodes(ctx context.Context) <-chan map[string]string {
	pubsub := cluster.redis.Subscribe(ctx, cluster.key(ClusterRegisterNodeChannel), cluster.key(ClusterReleaseNodeChannel))
	sink := make(chan map[string]string, 1)

	go func() {
		for range pubsub.Channel() {
			nodes, err := cluster.GetNodes(ctx)
			if err != nil {
				continue
			}

			sink <- nodes
		}
	}()

	return sink
}

func (cluster *Cluster) GetNodes(ctx context.Context) (map[string]string, error) {
	pattern := cluster.key("cluster:node:*")
	keys, err := cluster.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	values, err := cluster.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	prefixLen := len(cluster.key("cluster:node:"))
	nodes := make(map[string]string, len(values))
	for index, key := range keys {
		if values[index] == nil {
			continue // skip nil values
		}

		nodeID := key[prefixLen:]
		nodes[nodeID] = values[index].(string)
	}

	return nodes, nil
}

func (cluster *Cluster) Lock(ctx context.Context) time.Time {
	err := cluster.mu.TryLockContext(ctx)
	if err != nil {
		return cluster.mu.Until()
	}

	return cluster.mu.Until()
}

func (cluster *Cluster) Extend(ctx context.Context) time.Time {
	_, err := cluster.mu.ExtendContext(ctx)
	if err != nil {
		return cluster.mu.Until()
	}

	return cluster.mu.Until()
}

func (cluster *Cluster) Unlock(ctx context.Context) error {
	_, err := cluster.mu.UnlockContext(ctx)
	if errors.Is(err, redsync.ErrLockAlreadyExpired) || errors.Is(err, redsync.ErrNodeTaken{}) {
		return nil
	}

	return err
}
