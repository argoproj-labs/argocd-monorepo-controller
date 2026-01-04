package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsServer struct {
	handler                           http.Handler
	gitFetchFailCounter               *prometheus.CounterVec
	gitLsRemoteFailCounter            *prometheus.CounterVec
	gitDiffTreeFailCounter            *prometheus.CounterVec
	gitRevListFailCounter             *prometheus.CounterVec
	gitRequestCounter                 *prometheus.CounterVec
	gitRequestHistogram               *prometheus.HistogramVec
	getChangeRevisionRequestCounter   *prometheus.CounterVec
	getChangeRevisionRequestHistogram *prometheus.HistogramVec
	repoPendingRequestsGauge          *prometheus.GaugeVec
	redisRequestCounter               *prometheus.CounterVec
	redisRequestHistogram             *prometheus.HistogramVec
	PrometheusRegistry                *prometheus.Registry
}

type GitRequestType string

const (
	GitRequestTypeLsRemote = "ls-remote"
	GitRequestTypeFetch    = "fetch"
	GitRequestTypeDiffTree = "diff-tree"
	GitRequestTypeRevList  = "rev-list"
)

// NewMetricsServer returns a new prometheus server which collects application metrics.
func NewMetricsServer() *MetricsServer {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	getChangeRevisionRequestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monorepo_getchangerevision_request_total",
			Help: "Number of GetChangeRevision requests executed.",
		},
		[]string{"repo", "failed", "application", "namespace"},
	)
	registry.MustRegister(getChangeRevisionRequestCounter)

	getChangeRevisionRequestHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "monorepo_getchangerevision_request_duration_seconds",
			Help:    "GetChangeRevision requests duration seconds.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2, 4, 10, 20},
		},
		[]string{"repo", "application", "namespace"},
	)
	registry.MustRegister(getChangeRevisionRequestHistogram)

	gitFetchFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monorepo_git_fetch_fail_total",
			Help: "Number of git fetch requests failures by repo server",
		},
		[]string{"repo", "revision"},
	)
	registry.MustRegister(gitFetchFailCounter)

	gitLsRemoteFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monorepo_git_lsremote_fail_total",
			Help: "Number of git ls-remote requests failures by repo server",
		},
		[]string{"repo", "revision"},
	)
	registry.MustRegister(gitLsRemoteFailCounter)

	gitRevListFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monorepo_git_revlist_fail_total",
			Help: "Number of git rev-list requests failures by repo server",
		},
		[]string{"repo", "app", "namespace"},
	)
	registry.MustRegister(gitRevListFailCounter)

	gitDiffTreeFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monorepo_git_difftree_fail_total",
			Help: "Number of git diff-tree requests failures by repo server",
		},
		[]string{"repo", "app", "namespace"},
	)
	registry.MustRegister(gitDiffTreeFailCounter)

	gitRequestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monorepo_git_request_total",
			Help: "Number of git requests performed by repo server",
		},
		[]string{"repo", "request_type"},
	)
	registry.MustRegister(gitRequestCounter)

	gitRequestHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "monorepo_git_request_duration_seconds",
			Help:    "Git requests duration seconds.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2, 4, 10, 20},
		},
		[]string{"repo", "request_type"},
	)
	registry.MustRegister(gitRequestHistogram)

	repoPendingRequestsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "monorepo_repo_pending_request_total",
			Help: "Number of pending requests requiring repository lock",
		},
		[]string{"repo"},
	)
	registry.MustRegister(repoPendingRequestsGauge)

	redisRequestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monorepo_redis_request_total",
			Help: "Number of redis requests executed.",
		},
		[]string{"initiator", "failed"},
	)
	registry.MustRegister(redisRequestCounter)

	redisRequestHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "monorepo_redis_request_duration_seconds",
			Help:    "Redis requests duration seconds.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2},
		},
		[]string{"initiator"},
	)
	registry.MustRegister(redisRequestHistogram)

	return &MetricsServer{
		handler:                           promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		gitFetchFailCounter:               gitFetchFailCounter,
		gitLsRemoteFailCounter:            gitLsRemoteFailCounter,
		gitRevListFailCounter:             gitRevListFailCounter,
		gitDiffTreeFailCounter:            gitDiffTreeFailCounter,
		getChangeRevisionRequestCounter:   getChangeRevisionRequestCounter,
		getChangeRevisionRequestHistogram: getChangeRevisionRequestHistogram,
		gitRequestCounter:                 gitRequestCounter,
		gitRequestHistogram:               gitRequestHistogram,
		repoPendingRequestsGauge:          repoPendingRequestsGauge,
		redisRequestCounter:               redisRequestCounter,
		redisRequestHistogram:             redisRequestHistogram,
		PrometheusRegistry:                registry,
	}
}

func (m *MetricsServer) GetHandler() http.Handler {
	return m.handler
}

func (m *MetricsServer) IncGitFetchFail(repo string, revision string) {
	m.gitFetchFailCounter.WithLabelValues(repo, revision).Inc()
}

func (m *MetricsServer) IncGitLsRemoteFail(repo string, revision string) {
	m.gitLsRemoteFailCounter.WithLabelValues(repo, revision).Inc()
}

func (m *MetricsServer) IncDiffTreeFail(repo string, app string, namespace string) {
	m.gitDiffTreeFailCounter.WithLabelValues(repo, app, namespace).Inc()
}

func (m *MetricsServer) IncRevListFail(repo string, app string, namespace string) {
	m.gitRevListFailCounter.WithLabelValues(repo, app, namespace).Inc()
}

// IncGitRequest increments the git requests counter
func (m *MetricsServer) IncGitRequest(repo string, requestType GitRequestType) {
	m.gitRequestCounter.WithLabelValues(repo, string(requestType)).Inc()
}

func (m *MetricsServer) IncPendingRepoRequest(repo string) {
	m.repoPendingRequestsGauge.WithLabelValues(repo).Inc()
}

func (m *MetricsServer) ObserveGitRequestDuration(repo string, requestType GitRequestType, duration time.Duration) {
	m.gitRequestHistogram.WithLabelValues(repo, string(requestType)).Observe(duration.Seconds())
}

func (m *MetricsServer) DecPendingRepoRequest(repo string) {
	m.repoPendingRequestsGauge.WithLabelValues(repo).Dec()
}

func (m *MetricsServer) IncRedisRequest(failed bool) {
	m.redisRequestCounter.WithLabelValues("argocd-repo-server", strconv.FormatBool(failed)).Inc()
}

func (m *MetricsServer) ObserveRedisRequestDuration(duration time.Duration) {
	m.redisRequestHistogram.WithLabelValues("argocd-repo-server").Observe(duration.Seconds())
}

func (m *MetricsServer) IncGetChangeRevisionRequest(repo string, failed bool, app string, namespace string) {
	m.getChangeRevisionRequestCounter.WithLabelValues(repo, strconv.FormatBool(failed), app, namespace).Inc()
}

func (m *MetricsServer) ObserveGetChangeRevisionRequestDuration(duration time.Duration, repo string, app string, namespace string) {
	m.getChangeRevisionRequestHistogram.WithLabelValues(repo, app, namespace).Observe(duration.Seconds())
}
