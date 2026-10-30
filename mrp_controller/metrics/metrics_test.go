package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestMetricsServer builds the part of the server the retry metrics need,
// without the HTTP listener and the application lister a whole one requires.
// The metric vectors are package globals, so they are reset for every test.
func newTestMetricsServer() *MetricsServer {
	appRetryCounter.Reset()
	appRetryExhaustedCounter.Reset()
	retryQueueLengthGauge.Reset()
	return &MetricsServer{
		hostname:                 "test-host",
		appRetryCounter:          appRetryCounter,
		appRetryExhaustedCounter: appRetryExhaustedCounter,
	}
}

func TestMetricsServer_IncAppRetry(t *testing.T) {
	m := newTestMetricsServer()

	m.IncAppRetry("argocd", "app-1")
	m.IncAppRetry("argocd", "app-1")
	m.IncAppRetry("other-ns", "app-2")

	assert.InDelta(t, 2.0, testutil.ToFloat64(appRetryCounter.WithLabelValues("argocd", "app-1")), 0)
	assert.InDelta(t, 1.0, testutil.ToFloat64(appRetryCounter.WithLabelValues("other-ns", "app-2")), 0)
}

func TestMetricsServer_IncAppRetryExhausted(t *testing.T) {
	m := newTestMetricsServer()

	m.IncAppRetryExhausted("argocd", "app-1")

	assert.InDelta(t, 1.0, testutil.ToFloat64(appRetryExhaustedCounter.WithLabelValues("argocd", "app-1")), 0)
	assert.InDelta(t, 0.0, testutil.ToFloat64(appRetryCounter.WithLabelValues("argocd", "app-1")), 0,
		"giving up on an application is not a retry")
}

func TestMetricsServer_SetRetryQueueLength(t *testing.T) {
	m := newTestMetricsServer()

	m.SetRetryQueueLength(3)
	assert.InDelta(t, 3.0, testutil.ToFloat64(retryQueueLengthGauge.WithLabelValues("test-host")), 0)

	m.SetRetryQueueLength(0)
	assert.InDelta(t, 0.0, testutil.ToFloat64(retryQueueLengthGauge.WithLabelValues("test-host")), 0)
}

// TestMetricsServer_RetryMetricsAreExposed pins the names, the help texts and
// the labels the metrics are scraped under, which the dashboards depend on.
func TestMetricsServer_RetryMetricsAreExposed(t *testing.T) {
	m := newTestMetricsServer()

	m.IncAppRetry("argocd", "app-1")
	m.IncAppRetryExhausted("argocd", "app-1")
	m.SetRetryQueueLength(2)

	expected := `
# HELP monorepo_app_retry_total Number of retries scheduled for applications whose handling failed with a retryable error.
# TYPE monorepo_app_retry_total counter
monorepo_app_retry_total{application="app-1",namespace="argocd"} 1
`
	require.NoError(t, testutil.CollectAndCompare(appRetryCounter, strings.NewReader(expected)))

	expected = `
# HELP monorepo_app_retry_exhausted_total Number of applications given up on because all the retries allowed have failed.
# TYPE monorepo_app_retry_exhausted_total counter
monorepo_app_retry_exhausted_total{application="app-1",namespace="argocd"} 1
`
	require.NoError(t, testutil.CollectAndCompare(appRetryExhaustedCounter, strings.NewReader(expected)))

	expected = `
# HELP monorepo_app_retry_queue_length Number of applications waiting for their retry delay to pass.
# TYPE monorepo_app_retry_queue_length gauge
monorepo_app_retry_queue_length{hostname="test-host"} 2
`
	require.NoError(t, testutil.CollectAndCompare(retryQueueLengthGauge, strings.NewReader(expected)))
}
