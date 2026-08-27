package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ============================================================================
// Cluster
// ============================================================================

var ClusterLeader = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "lunogram_cluster_leader",
	Help: "Indicates if this node is the cluster leader (1 for leader, 0 for not)",
})

var TotalNodes = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "lunogram_nodes",
	Help: "The total amount of nodes within the cluster",
})

var LeaderElectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "lunogram_leader_elections_total",
	Help: "Total number of leader elections won by this node",
})

var LeaderElectionFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "lunogram_leader_election_failures_total",
	Help: "Total number of failed leadership extensions",
})

// ============================================================================
// Journey Progression
// ============================================================================

var JourneyStepsProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_journey_steps_processed_total",
	Help: "Total journey steps processed successfully",
}, []string{"step_type", "project_id"})

var JourneyStepsErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_journey_steps_errors_total",
	Help: "Total journey step processing failures",
}, []string{"step_type", "project_id"})

var JourneyStepDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "lunogram_journey_step_duration_seconds",
	Help:    "Duration of journey step processing in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"step_type", "project_id"})

var JourneyStepsPausedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_journey_steps_paused_total",
	Help: "Total journey steps that entered a paused/waiting state",
}, []string{"step_type", "project_id"})

var JourneyStepsCompletedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_journey_steps_completed_total",
	Help: "Total journey steps that completed successfully",
}, []string{"step_type", "project_id"})

var JourneyEntrancesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_journey_entrances_total",
	Help: "Total new journey entrances triggered by events",
}, []string{"project_id"})

var JourneyEntranceRejectionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_journey_entrance_rejections_total",
	Help: "Total journey entrance rejections due to eligibility checks",
}, []string{"project_id", "reason"})

var JourneyExitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_journey_exits_total",
	Help: "Total journey completions (user reached exit step)",
}, []string{"project_id"})

var JourneyGateEvaluationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_journey_gate_evaluations_total",
	Help: "Total gate step evaluations by branch result",
}, []string{"project_id", "result"})

var JourneyExperimentAssignmentsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_journey_experiment_assignments_total",
	Help: "Total experiment/A-B test branch assignments",
}, []string{"project_id", "branch"})

// ============================================================================
// Reconciliation (Leader Scheduler)
// ============================================================================

var ReconciliationDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "lunogram_reconciliation_duration_seconds",
	Help:    "Duration of each reconciliation task in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"task"})

var ReconciliationRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_reconciliation_runs_total",
	Help: "Total reconciliation cycles run per task",
}, []string{"task"})

var ReconciliationItemsProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_reconciliation_items_processed_total",
	Help: "Total items scanned during reconciliation",
}, []string{"task"})

var ReconciliationItemsPublishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_reconciliation_items_published_total",
	Help: "Total items successfully published during reconciliation",
}, []string{"task"})

var ReconciliationItemsFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_reconciliation_items_failed_total",
	Help: "Total items that failed during reconciliation",
}, []string{"task"})

var ReconciliationPanicsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_reconciliation_panics_total",
	Help: "Total panics caught during reconciliation tasks",
}, []string{"task"})

// ============================================================================
// Scheduled Events
// ============================================================================

var ScheduledEventsFiredTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_scheduled_events_fired_total",
	Help: "Total scheduled events that fired on time",
}, []string{"subject_type"})

var ScheduledEventsFireFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_scheduled_events_fire_failures_total",
	Help: "Total scheduled events that failed to fire",
}, []string{"subject_type"})

var ScheduledEventsFireDelaySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "lunogram_scheduled_events_fire_delay_seconds",
	Help:    "Delay between intended fire time and actual fire time in seconds",
	Buckets: []float64{0.5, 1, 5, 10, 30, 60, 120, 300, 600},
}, []string{"subject_type"})

var SchedulesAdvancedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_schedules_advanced_total",
	Help: "Total recurring schedules advanced to next cycle",
}, []string{"subject_type"})

var SchedulesAdvanceFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_schedules_advance_failures_total",
	Help: "Total failed schedule advancements",
}, []string{"subject_type"})

var ScheduledEventsIngestedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_scheduled_events_ingested_total",
	Help: "Total new schedule assignments ingested",
}, []string{"subject_type", "schedule_type"})

// ============================================================================
// Triggered Events (Real-Time)
// ============================================================================

var EventsProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_events_processed_total",
	Help: "Total events ingested and processed",
}, []string{"subject_type"})

var EventsProcessingErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_events_processing_errors_total",
	Help: "Total event processing failures",
}, []string{"subject_type"})

var EventsProcessingDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "lunogram_events_processing_duration_seconds",
	Help:    "Duration of event processing in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"subject_type"})

var EventsJourneyTriggersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_events_journey_triggers_total",
	Help: "Total events that triggered journey entrances",
}, []string{"subject_type"})

var EventsListRecomputesTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "lunogram_events_list_recomputes_total",
	Help: "Total events that triggered list recomputation",
})

// ============================================================================
// Match Event Fan-Out
// ============================================================================

var MatchEventsProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_match_events_processed_total",
	Help: "Total match event messages processed (one per match message)",
}, []string{"subject_type"})

var MatchEventsMatchedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_match_events_matched_total",
	Help: "Total individual subjects matched and fanned out from match events",
}, []string{"subject_type"})

// ============================================================================
// NATS Message Processing (Router)
// ============================================================================

var NATSMessagesAckedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_nats_messages_acked_total",
	Help: "Total NATS messages successfully acknowledged",
}, []string{"stream", "consumer"})

var NATSMessagesNackedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_nats_messages_nacked_total",
	Help: "Total NATS messages negatively acknowledged for retry",
}, []string{"stream", "consumer"})

var NATSMessagesTerminatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_nats_messages_terminated_total",
	Help: "Total NATS messages permanently failed and terminated",
}, []string{"stream", "consumer"})

var NATSMessagesRateLimitedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_nats_messages_rate_limited_total",
	Help: "Total NATS messages rate-limited and re-scheduled for later delivery",
}, []string{"stream", "consumer"})

var NATSMessageProcessingDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "lunogram_nats_message_processing_duration_seconds",
	Help:    "Duration of NATS message processing in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"stream", "consumer"})

// ============================================================================
// Auth
// ============================================================================

// AuthLegacySessionUpgradeTotal counts requests whose legacy provider-issued
// session cookie was exchanged in flight for a Lunogram console session. It is
// the deletion criterion for that transitional path: once it has been flat at
// zero for a full legacy-cookie lifetime, no browser is holding one any more.
var AuthLegacySessionUpgradeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "lunogram_auth_legacy_session_upgrade_total",
	Help: "Total legacy session cookies exchanged for a Lunogram console session",
}, []string{"result"})

func NewHandler() http.Handler {
	return promhttp.Handler()
}
