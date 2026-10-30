package controller

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/mrp_controller/service"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

/* retry policy tests */

func TestRetryPolicy_BackoffGrowsExponentially(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 10, BaseDelay: 10 * time.Second, MaxDelay: 5 * time.Minute, Factor: 2}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: 10 * time.Second}, // out of range, treated as the first
		{attempt: 1, want: 10 * time.Second},
		{attempt: 2, want: 20 * time.Second},
		{attempt: 3, want: 40 * time.Second},
		{attempt: 4, want: 80 * time.Second},
		{attempt: 5, want: 160 * time.Second},
		{attempt: 6, want: 5 * time.Minute}, // capped
		{attempt: 100, want: 5 * time.Minute},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, policy.Backoff(tt.attempt), "attempt %d", tt.attempt)
	}
}

func TestRetryPolicy_BackoffWithoutCapDoesNotOverflow(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 100, BaseDelay: time.Second, Factor: 2}

	assert.Positive(t, policy.Backoff(1000))
	assert.LessOrEqual(t, policy.Backoff(1000), time.Duration(1<<63-1))
}

func TestRetryPolicy_BackoffJitterStaysWithinBounds(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 10, BaseDelay: 10 * time.Second, MaxDelay: time.Minute, Factor: 2, Jitter: 0.2}

	values := make(map[time.Duration]struct{})
	for range 100 {
		delay := policy.Backoff(2)
		assert.GreaterOrEqual(t, delay, 16*time.Second)
		assert.LessOrEqual(t, delay, 24*time.Second)
		values[delay] = struct{}{}

		// the jitter never pushes the delay over the cap
		assert.LessOrEqual(t, policy.Backoff(10), time.Minute)
	}
	assert.Greater(t, len(values), 1, "the delay must be randomized")
}

func TestRetryPolicy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		policy  RetryPolicy
		wantErr string
	}{
		{name: "default policy", policy: DefaultRetryPolicy()},
		{name: "retries disabled", policy: RetryPolicy{MaxAttempts: 0, BaseDelay: time.Second, Factor: 1}},
		{name: "delay not capped", policy: RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, Factor: 1}},
		{name: "negative max attempts", policy: RetryPolicy{MaxAttempts: -1, BaseDelay: time.Second, Factor: 1}, wantErr: "max attempts is -1"},
		{name: "zero base delay", policy: RetryPolicy{MaxAttempts: 1, Factor: 1}, wantErr: "base delay is 0s"},
		{name: "negative max delay", policy: RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, MaxDelay: -time.Second, Factor: 1}, wantErr: "max delay is -1s"},
		{name: "base delay over max delay", policy: RetryPolicy{MaxAttempts: 1, BaseDelay: time.Minute, MaxDelay: time.Second, Factor: 1}, wantErr: "base delay 1m0s is longer than max delay 1s"},
		{name: "shrinking backoff factor", policy: RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, Factor: 0.5}, wantErr: "backoff factor is 0.5"},
		{name: "jitter over one", policy: RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, Factor: 1, Jitter: 1.5}, wantErr: "jitter is 1.5"},
		{name: "negative jitter", policy: RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, Factor: 1, Jitter: -0.1}, wantErr: "jitter is -0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestRetryPolicy_ValidateReportsEveryProblem(t *testing.T) {
	err := RetryPolicy{MaxAttempts: -1, BaseDelay: -time.Second, MaxDelay: -time.Minute, Factor: 0, Jitter: 2}.Validate()

	require.ErrorContains(t, err, "max attempts is -1")
	require.ErrorContains(t, err, "base delay is -1s")
	require.ErrorContains(t, err, "max delay is -1m0s")
	require.ErrorContains(t, err, "backoff factor is 0")
	require.ErrorContains(t, err, "jitter is 2")
}

func TestRetryPolicy_Enabled(t *testing.T) {
	assert.True(t, DefaultRetryPolicy().Enabled())
	assert.False(t, RetryPolicy{}.Enabled())
	assert.False(t, RetryPolicy{MaxAttempts: -1}.Enabled())
}

/* queue retry tests */

// fakeTimers records the retries the queue schedules instead of waiting for
// them, so that a test can run them when it wants to.
type fakeTimers struct {
	mu        sync.Mutex
	scheduled []*fakeTimer
}

type fakeTimer struct {
	timers  *fakeTimers
	delay   time.Duration
	run     func()
	stopped bool
}

func (t *fakeTimer) Stop() bool {
	t.timers.mu.Lock()
	defer t.timers.mu.Unlock()
	wasRunning := !t.stopped
	t.stopped = true
	return wasRunning
}

func (f *fakeTimers) factory() timerFactory {
	return func(d time.Duration, run func()) retryTimer {
		f.mu.Lock()
		defer f.mu.Unlock()
		timer := &fakeTimer{timers: f, delay: d, run: run}
		f.scheduled = append(f.scheduled, timer)
		return timer
	}
}

// delays returns the delays of the retries which are still scheduled.
func (f *fakeTimers) delays() []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	var delays []time.Duration
	for _, timer := range f.scheduled {
		if !timer.stopped {
			delays = append(delays, timer.delay)
		}
	}
	return delays
}

// fire runs the retries which are still scheduled and reports how many of them
// there were. Like a real timer, the scheduled function is run without holding
// any of the locks of the timer itself.
func (f *fakeTimers) fire() int {
	f.mu.Lock()
	var due []*fakeTimer
	for _, timer := range f.scheduled {
		if !timer.stopped {
			due = append(due, timer)
		}
	}
	f.scheduled = nil
	f.mu.Unlock()

	for _, timer := range due {
		timer.run()
	}
	return len(due)
}

func newRetryTestQueue(policy RetryPolicy) (*KeyedAppQueue, *fakeTimers) {
	timers := &fakeTimers{}
	return NewKeyedAppQueue(WithRetryPolicy(policy), withTimerFactory(timers.factory())), timers
}

func testRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, BaseDelay: 10 * time.Second, MaxDelay: 5 * time.Minute, Factor: 2}
}

func retryableError() error {
	return status.Error(codes.Unavailable, "connection refused")
}

// queueState reports the number of the queued and the delayed apps, and the
// number of the attempts recorded for the given key.
func queueState(queue *KeyedAppQueue, key string) (queued, delayed, attempts int) {
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	return queue.items.Len(), len(queue.delayed), queue.budgets[key].attempts
}

func TestKeyedAppQueue_RetryableFailureIsRequeuedAfterBackoff(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v1"))
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())

	// the app is not queued yet, it waits for the backoff to pass
	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 0, queued)
	assert.Equal(t, 1, delayed)
	assert.Equal(t, 1, attempts)
	assert.Equal(t, []time.Duration{10 * time.Second}, timers.delays())

	assert.Equal(t, 1, timers.fire())

	queued, delayed, _ = queueState(queue, item.Key)
	assert.Equal(t, 1, queued)
	assert.Equal(t, 0, delayed)

	requeued := dequeue(t, queue)
	assert.Equal(t, "ns/app", requeued.Key)
	assert.Equal(t, "v1", requeued.Value.Status.Sync.Revision, "the retry handles the same event")
}

func TestKeyedAppQueue_RetryDelayGrowsWithEveryAttempt(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())
	queue.Enqueue(createTestApplication("ns", "app"))

	for _, want := range []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second} {
		item := dequeue(t, queue)
		queue.DoneWithError(item.Key, item.Value, retryableError())
		assert.Equal(t, []time.Duration{want}, timers.delays())
		require.Equal(t, 1, timers.fire())
	}

	// the attempts allowed by the policy are used up now
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())

	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 0, queued)
	assert.Equal(t, 0, delayed, "the application is given up on")
	assert.Equal(t, 0, attempts, "the budget of the next event is not affected")
	assert.Empty(t, timers.delays())
}

func TestKeyedAppQueue_PermanentFailureIsNotRetried(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	queue.Enqueue(createTestApplication("ns", "app"))
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, service.NewPermanentError(errors.New("no manifest generate paths")))

	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 0, queued)
	assert.Equal(t, 0, delayed)
	assert.Equal(t, 0, attempts)
	assert.Empty(t, timers.delays())
}

func TestKeyedAppQueue_SuccessResetsTheRetryBudget(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())
	queue.Enqueue(createTestApplication("ns", "app"))

	// two failed attempts
	for range 2 {
		item := dequeue(t, queue)
		queue.DoneWithError(item.Key, item.Value, retryableError())
		require.Equal(t, 1, timers.fire())
	}

	// followed by a successful one
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, nil)
	_, _, attempts := queueState(queue, item.Key)
	assert.Equal(t, 0, attempts)

	// so the next failure starts the backoff from the beginning again
	queue.Enqueue(createTestApplication("ns", "app"))
	item = dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	assert.Equal(t, []time.Duration{10 * time.Second}, timers.delays())
}

func TestKeyedAppQueue_RetriesDisabled(t *testing.T) {
	queue, timers := newRetryTestQueue(RetryPolicy{})

	queue.Enqueue(createTestApplication("ns", "app"))
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())

	queued, delayed, _ := queueState(queue, item.Key)
	assert.Equal(t, 0, queued)
	assert.Equal(t, 0, delayed)
	assert.Empty(t, timers.delays())
}

func TestKeyedAppQueue_NewEventSupersedesScheduledRetry(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v1"))
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	require.Len(t, timers.delays(), 1)

	// the event which arrives while the retry waits replaces it
	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v2"))

	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 1, queued)
	assert.Equal(t, 0, delayed)
	assert.Equal(t, 0, attempts, "a new event is given a new retry budget")
	assert.Empty(t, timers.delays(), "the scheduled retry is cancelled")

	requeued := dequeue(t, queue)
	assert.Equal(t, "v2", requeued.Value.Status.Sync.Revision)

	// the cancelled retry does not queue the stale event when its timer fires
	assert.Equal(t, 0, timers.fire())
	queued, delayed, _ = queueState(queue, item.Key)
	assert.Equal(t, 0, queued)
	assert.Equal(t, 0, delayed)
}

func TestKeyedAppQueue_FiringTimerOfCancelledRetryIsIgnored(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v1"))
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())

	// the timer of the first retry is already running when the retry is
	// cancelled and the app is scheduled again by a later attempt
	stale := timers.scheduled[0]
	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v2"))
	second := dequeue(t, queue)
	queue.DoneWithError(second.Key, second.Value, retryableError())

	stale.run()

	queued, delayed, _ := queueState(queue, item.Key)
	assert.Equal(t, 0, queued, "the cancelled retry must not requeue the application")
	assert.Equal(t, 1, delayed, "the current retry is still scheduled")
}

func TestKeyedAppQueue_PendingEventSupersedesRetry(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v1"))
	item := dequeue(t, queue)

	// an event received while the app is handled wins over the retry
	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v2"))
	queue.DoneWithError(item.Key, item.Value, retryableError())

	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 1, queued)
	assert.Equal(t, 0, delayed)
	assert.Equal(t, 0, attempts)
	assert.Empty(t, timers.delays())

	requeued := dequeue(t, queue)
	assert.Equal(t, "v2", requeued.Value.Status.Sync.Revision)
}

func TestKeyedAppQueue_ForgetAppDropsScheduledRetry(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	queue.Enqueue(createTestApplication("ns", "app"))
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	require.Len(t, timers.delays(), 1)

	queue.ForgetApp(item.Key)

	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 0, queued)
	assert.Equal(t, 0, delayed)
	assert.Equal(t, 0, attempts)
	assert.Empty(t, timers.delays())

	assert.Equal(t, 0, timers.fire())
	queued, _, _ = queueState(queue, item.Key)
	assert.Equal(t, 0, queued, "a deleted application is not requeued")
}

func TestKeyedAppQueue_ScheduledRetryIsNotCountedAsQueueLength(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())
	gauge := &testGauge{}
	queue.SetLengthGauge(gauge)

	queue.Enqueue(createTestApplication("ns", "app"))
	assert.InDelta(t, float64(1), gauge.lastValue, 0)

	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	assert.InDelta(t, float64(0), gauge.lastValue, 0, "an application waiting for its retry is not queued")

	require.Equal(t, 1, timers.fire())
	assert.InDelta(t, float64(1), gauge.lastValue, 0)
}

/* informer resync tests */

// createTestApplicationWithVersion builds the application as the informer
// delivers it, with the resource version the queue tells a redelivery by.
func createTestApplicationWithVersion(namespace, name, resourceVersion string) appv1.Application {
	app := createTestApplication(namespace, name)
	app.ResourceVersion = resourceVersion
	return app
}

func TestKeyedAppQueue_ResyncDoesNotDisturbAScheduledRetry(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	require.Equal(t, []time.Duration{10 * time.Second}, timers.delays())

	// the resync redelivers the very same application
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))

	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 0, queued, "the redelivery is not queued, the retry is already waiting")
	assert.Equal(t, 1, delayed)
	assert.Equal(t, 1, attempts, "the redelivery does not hand out a new budget")
	assert.Equal(t, []time.Duration{10 * time.Second}, timers.delays(), "the scheduled retry is untouched")
}

func TestKeyedAppQueue_ResyncDoesNotRestartTheBackoff(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))

	// every attempt is preceded by a resync of the same application, which
	// must not stop the application from being given up on
	for _, want := range []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second} {
		item := dequeue(t, queue)
		queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))
		queue.DoneWithError(item.Key, item.Value, retryableError())
		assert.Equal(t, []time.Duration{want}, timers.delays())
		require.Equal(t, 1, timers.fire())
	}

	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())

	_, delayed, _ := queueState(queue, item.Key)
	assert.Equal(t, 0, delayed, "the budget was used up despite the resyncs")
	assert.Empty(t, timers.delays())
}

func TestKeyedAppQueue_ResyncRequeuesAnApplicationGivenUpOn(t *testing.T) {
	queue, timers := newRetryTestQueue(RetryPolicy{MaxAttempts: 1, BaseDelay: 10 * time.Second, Factor: 2})
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))

	// use up the single attempt the policy allows
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	require.Equal(t, 1, timers.fire())
	item = dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())

	queued, delayed, attempts := queueState(queue, item.Key)
	require.Equal(t, 0, queued)
	require.Equal(t, 0, delayed)
	require.Equal(t, 0, attempts)

	// the resync is the only thing which gets the application looked at again
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))

	queued, _, _ = queueState(queue, item.Key)
	assert.Equal(t, 1, queued, "an application nothing is scheduled for is handled again")
}

func TestKeyedAppQueue_NewVersionGetsANewBudget(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	require.Len(t, timers.delays(), 1)

	// a real update, not a redelivery
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "101"))

	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 1, queued)
	assert.Equal(t, 0, delayed, "the scheduled retry is superseded")
	assert.Equal(t, 0, attempts, "the new version starts with a full budget")
	assert.Empty(t, timers.delays())

	requeued := dequeue(t, queue)
	assert.Equal(t, "101", requeued.Value.ResourceVersion)
}

func TestKeyedAppQueue_ResyncWhileProcessingDoesNotRestartTheBackoff(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))

	item := dequeue(t, queue)
	// the resync lands while the application is being handled, so it is
	// recorded as a pending update
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))
	queue.DoneWithError(item.Key, item.Value, retryableError())

	// the redelivery brought nothing the attempt did not already have, so the
	// failure is retried as if it had never arrived
	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 0, queued, "the redelivery is not handled again right away")
	assert.Equal(t, 1, delayed)
	assert.Equal(t, 1, attempts, "the failed attempt is counted")
	assert.Equal(t, []time.Duration{10 * time.Second}, timers.delays())

	require.Equal(t, 1, timers.fire())
	item = dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	assert.Equal(t, []time.Duration{20 * time.Second}, timers.delays(), "the backoff continued where it left off")
}

func TestKeyedAppQueue_ResyncOfASucceededApplicationIsNotHandledAgain(t *testing.T) {
	queue, _ := newRetryTestQueue(testRetryPolicy())
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))

	item := dequeue(t, queue)
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))
	queue.DoneWithError(item.Key, item.Value, nil)

	queued, _, _ := queueState(queue, item.Key)
	assert.Equal(t, 0, queued, "the version which was just handled is not handled again")
}

func TestKeyedAppQueue_PendingUpdateOfANewVersionIsStillHandled(t *testing.T) {
	queue, _ := newRetryTestQueue(testRetryPolicy())
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "100"))

	item := dequeue(t, queue)
	queue.Enqueue(createTestApplicationWithVersion("ns", "app", "101"))
	queue.DoneWithError(item.Key, item.Value, nil)

	queued, _, _ := queueState(queue, item.Key)
	require.Equal(t, 1, queued)
	assert.Equal(t, "101", dequeue(t, queue).Value.ResourceVersion)
}

func TestKeyedAppQueue_ApplicationsWithoutResourceVersionAreAlwaysNew(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	queue.Enqueue(createTestApplication("ns", "app"))
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	require.Len(t, timers.delays(), 1)

	queue.Enqueue(createTestApplication("ns", "app"))

	queued, delayed, attempts := queueState(queue, item.Key)
	assert.Equal(t, 1, queued)
	assert.Equal(t, 0, delayed)
	assert.Equal(t, 0, attempts)
}

func TestSameVersion(t *testing.T) {
	versioned := createTestApplicationWithVersion("ns", "app", "100")
	other := createTestApplicationWithVersion("ns", "app", "101")
	unversioned := createTestApplication("ns", "app")

	assert.True(t, sameVersion(&versioned, &versioned))
	assert.False(t, sameVersion(&versioned, &other))
	assert.False(t, sameVersion(&unversioned, &unversioned), "an application with no version is never a redelivery")
	assert.False(t, sameVersion(&versioned, &unversioned))
}

/* retry metrics tests */

// testRetryMetrics records what the queue reports about the retries.
type testRetryMetrics struct {
	mu          sync.Mutex
	retries     map[string]int
	exhausted   map[string]int
	queueLength int
}

func newTestRetryMetrics() *testRetryMetrics {
	return &testRetryMetrics{
		retries:     make(map[string]int),
		exhausted:   make(map[string]int),
		queueLength: -1,
	}
}

func (m *testRetryMetrics) IncAppRetry(namespace, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retries[namespace+"/"+name]++
}

func (m *testRetryMetrics) IncAppRetryExhausted(namespace, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exhausted[namespace+"/"+name]++
}

func (m *testRetryMetrics) SetRetryQueueLength(length int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queueLength = length
}

func (m *testRetryMetrics) counts(key string) (retries, exhausted, queueLength int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.retries[key], m.exhausted[key], m.queueLength
}

func newRetryTestQueueWithMetrics(policy RetryPolicy) (*KeyedAppQueue, *fakeTimers, *testRetryMetrics) {
	timers := &fakeTimers{}
	metrics := newTestRetryMetrics()
	queue := NewKeyedAppQueue(WithRetryPolicy(policy), withTimerFactory(timers.factory()), WithRetryMetrics(metrics))
	return queue, timers, metrics
}

func TestKeyedAppQueue_RetryQueueLengthIsReportedOnCreation(t *testing.T) {
	_, _, metrics := newRetryTestQueueWithMetrics(testRetryPolicy())

	_, _, queueLength := metrics.counts("ns/app")
	assert.Equal(t, 0, queueLength, "the empty retry queue must be reported, not left unset")
}

func TestKeyedAppQueue_RetryMetricsCountAttemptsAndExhaustion(t *testing.T) {
	queue, timers, metrics := newRetryTestQueueWithMetrics(testRetryPolicy())
	queue.Enqueue(createTestApplication("ns", "app"))

	// the three attempts the policy allows
	for attempt := 1; attempt <= 3; attempt++ {
		item := dequeue(t, queue)
		queue.DoneWithError(item.Key, item.Value, retryableError())

		retries, exhausted, queueLength := metrics.counts("ns/app")
		assert.Equal(t, attempt, retries)
		assert.Equal(t, 0, exhausted)
		assert.Equal(t, 1, queueLength, "the application waits for its retry delay")

		require.Equal(t, 1, timers.fire())
		_, _, queueLength = metrics.counts("ns/app")
		assert.Equal(t, 0, queueLength, "the application left the retry queue")
	}

	// the budget is used up, so this failure gives up on the application
	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())

	retries, exhausted, queueLength := metrics.counts("ns/app")
	assert.Equal(t, 3, retries, "the attempt which was given up on is not a retry")
	assert.Equal(t, 1, exhausted)
	assert.Equal(t, 0, queueLength)
}

func TestKeyedAppQueue_RetryQueueLengthCountsEveryDelayedApp(t *testing.T) {
	queue, timers, metrics := newRetryTestQueueWithMetrics(testRetryPolicy())
	queue.Enqueue(createTestApplication("ns", "app-1"))
	queue.Enqueue(createTestApplication("ns", "app-2"))

	for range 2 {
		item := dequeue(t, queue)
		queue.DoneWithError(item.Key, item.Value, retryableError())
	}
	_, _, queueLength := metrics.counts("")
	assert.Equal(t, 2, queueLength)

	require.Equal(t, 2, timers.fire())
	_, _, queueLength = metrics.counts("")
	assert.Equal(t, 0, queueLength)
}

func TestKeyedAppQueue_CancelledRetryLeavesTheRetryQueue(t *testing.T) {
	queue, _, metrics := newRetryTestQueueWithMetrics(testRetryPolicy())
	queue.Enqueue(createTestApplication("ns", "app"))

	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())
	_, _, queueLength := metrics.counts("ns/app")
	require.Equal(t, 1, queueLength)

	queue.ForgetApp(item.Key)
	_, _, queueLength = metrics.counts("ns/app")
	assert.Equal(t, 0, queueLength)
}

func TestKeyedAppQueue_PermanentFailureIsNotCounted(t *testing.T) {
	queue, _, metrics := newRetryTestQueueWithMetrics(testRetryPolicy())
	queue.Enqueue(createTestApplication("ns", "app"))

	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, service.NewPermanentError(errors.New("no manifest generate paths")))

	retries, exhausted, queueLength := metrics.counts("ns/app")
	assert.Equal(t, 0, retries)
	assert.Equal(t, 0, exhausted, "an error which cannot succeed on a retry exhausts nothing")
	assert.Equal(t, 0, queueLength)
}

func TestKeyedAppQueue_DisabledRetriesExhaustNothing(t *testing.T) {
	queue, _, metrics := newRetryTestQueueWithMetrics(RetryPolicy{})
	queue.Enqueue(createTestApplication("ns", "app"))

	item := dequeue(t, queue)
	queue.DoneWithError(item.Key, item.Value, retryableError())

	retries, exhausted, _ := metrics.counts("ns/app")
	assert.Equal(t, 0, retries)
	assert.Equal(t, 0, exhausted, "a failure is not an exhausted retry budget when retries are off")
}

func TestSplitAppKey(t *testing.T) {
	tests := []struct {
		key           string
		wantNamespace string
		wantName      string
	}{
		{key: "argocd/my-app", wantNamespace: "argocd", wantName: "my-app"},
		{key: AppKey(&appv1.Application{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "app"}}), wantNamespace: "ns", wantName: "app"},
		{key: "no-separator", wantNamespace: "no-separator", wantName: ""},
		{key: "", wantNamespace: "", wantName: ""},
	}
	for _, tt := range tests {
		namespace, name := splitAppKey(tt.key)
		assert.Equal(t, tt.wantNamespace, namespace, "key %q", tt.key)
		assert.Equal(t, tt.wantName, name, "key %q", tt.key)
	}
}

func TestWorkerPool_FailedAppIsHandledAgain(t *testing.T) {
	queue, timers := newRetryTestQueue(testRetryPolicy())

	handled := make(chan string, 4)
	var attempts int
	var mu sync.Mutex
	handler := func(_ context.Context, a appv1.Application) error {
		mu.Lock()
		attempts++
		attempt := attempts
		mu.Unlock()
		handled <- a.Status.Sync.Revision
		if attempt == 1 {
			return retryableError()
		}
		return nil
	}

	pool := NewWorkerPool(queue, 1, handler)
	defer pool.Stop()

	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v1"))

	select {
	case revision := <-handled:
		assert.Equal(t, "v1", revision)
	case <-time.After(2 * time.Second):
		t.Fatal("the application was not handled")
	}

	// the failed attempt scheduled a retry, which the pool handles once its delay has passed
	require.Eventually(t, func() bool { return len(timers.delays()) == 1 }, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, 1, timers.fire())

	select {
	case revision := <-handled:
		assert.Equal(t, "v1", revision, "the retry handles the same event")
	case <-time.After(2 * time.Second):
		t.Fatal("the application was not handled again")
	}

	// the second attempt succeeded, so no further retry is scheduled
	require.Eventually(t, func() bool {
		_, _, attemptsLeft := queueState(queue, "ns/app")
		return attemptsLeft == 0
	}, 2*time.Second, 10*time.Millisecond)
	assert.Empty(t, timers.delays())
}
