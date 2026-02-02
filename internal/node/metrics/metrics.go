package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var ClusterLeader = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "lunogram_cluster_leader",
	Help: "Indicates if this node is the cluster leader (1 for leader, 0 for not)",
})

var TotalNodes = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "lunogram_nodes",
	Help: "The total amount of nodes within the cluster",
})

func NewHandler() http.Handler {
	return promhttp.Handler()
}
