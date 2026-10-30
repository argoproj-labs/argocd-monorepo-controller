package controller

import (
	"container/list"
	"context"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/mrp_controller/service"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type AppQueueItem struct {
	Key   string
	Value appv1.Application
}

func AppKey(app *appv1.Application) string {
	return app.Namespace + "/" + app.Name
}

func splitAppKey(key string) (namespace, name string) {
	namespace, name, _ = strings.Cut(key, "/")
	return namespace, name
}

// delayedRetry represents an application waiting for its retry delay to pass.
type delayedRetry struct {
	app   appv1.Application
	timer retryTimer
	gen   uint64
}

// retryBudget counts the retries made for an version of an application
type retryBudget struct {
	attempts        int
	resourceVersion string
}

type gaugeSetter interface {
	Set(float64)
}

type retryMetrics interface {
	// IncAppRetry counts a retry scheduled for the application.
	IncAppRetry(namespace, name string)
	// IncAppRetryExhausted counts an application given up on retries
	IncAppRetryExhausted(namespace, name string)
	// SetRetryQueueLength records applications waiting for their retry delay to pass.
	SetRetryQueueLength(length int)
}

type KeyedAppQueue struct {
	mu     sync.RWMutex
	items  *list.List
	keyMap map[string]*list.Element
	// currently handled keys
	processing map[string]struct{}
	// updates received during keys processing, re-queued by Done
	pending     map[string]appv1.Application
	notEmpty    *sync.Cond
	lengthGauge gaugeSetter
	// keys waiting to be requeued after a failed attempt to handle them
	delayed map[string]*delayedRetry
	// retries already made for a key, reset when the key is handled
	// successfully or a new version of the application is received
	budgets  map[string]retryBudget
	retry    RetryPolicy
	newTimer timerFactory
	metrics  retryMetrics
	// number of the retries scheduled so far, used to recognize a retry which
	// was cancelled while its timer was already firing
	retryGen uint64
}

// QueueOption configures the application queue.
type QueueOption func(*KeyedAppQueue)

// WithRetryPolicy makes the queue retry the applications whose handling failed
// with a retriable error
func WithRetryPolicy(policy RetryPolicy) QueueOption {
	return func(kq *KeyedAppQueue) {
		kq.retry = policy
	}
}

// WithRetryMetrics makes the queue report the retries it schedules.
func WithRetryMetrics(m retryMetrics) QueueOption {
	return func(kq *KeyedAppQueue) {
		kq.metrics = m
	}
}

// withTimerFactory replaces the way the retries are scheduled, for the tests.
func withTimerFactory(newTimer timerFactory) QueueOption {
	return func(kq *KeyedAppQueue) {
		kq.newTimer = newTimer
	}
}

func NewKeyedAppQueue(opts ...QueueOption) *KeyedAppQueue {
	kaq := &KeyedAppQueue{
		items:      list.New(),
		keyMap:     make(map[string]*list.Element),
		processing: make(map[string]struct{}),
		pending:    make(map[string]appv1.Application),
		delayed:    make(map[string]*delayedRetry),
		budgets:    make(map[string]retryBudget),
		newTimer:   realTimer,
	}
	kaq.notEmpty = sync.NewCond(&kaq.mu)
	for _, opt := range opts {
		opt(kaq)
	}
	// report the empty retry queue, so that the metric does not appear only
	// once something has failed
	kaq.reportRetryQueueLength()
	return kaq
}

func (kq *KeyedAppQueue) Enqueue(app appv1.Application) {
	kq.mu.Lock()
	defer kq.mu.Unlock()
	key := AppKey(&app)

	if retry, waiting := kq.delayed[key]; waiting && sameVersion(&retry.app, &app) {
		log.Debugf("application %q is already waiting for its retry, ignoring the redelivery", key)
		return
	}

	// A new event carries fresher data than a scheduled retry.
	kq.cancelRetryLocked(key)
	kq.resetBudgetLocked(key, &app)
	kq.enqueueLocked(app)
}

// ForgetApp drops the retry scheduled for the application, It is
// called when the application is deleted.
func (kq *KeyedAppQueue) ForgetApp(key string) {
	kq.mu.Lock()
	defer kq.mu.Unlock()
	kq.cancelRetryLocked(key)
	delete(kq.budgets, key)
}

func sameVersion(a, b *appv1.Application) bool {
	return a.ResourceVersion != "" && a.ResourceVersion == b.ResourceVersion
}

// resetBudgetLocked gives the application a new budget of retry attempts only if
// its version has changed (to avoid periodic resinc resetting retry attempts
// for failing applications),
func (kq *KeyedAppQueue) resetBudgetLocked(key string, app *appv1.Application) {
	budget, ok := kq.budgets[key]
	if ok && app.ResourceVersion != "" && budget.resourceVersion == app.ResourceVersion {
		return
	}
	delete(kq.budgets, key)
}

// scheduleRetryLocked requeues the app once the delay has passed.
func (kq *KeyedAppQueue) scheduleRetryLocked(key string, app appv1.Application, delay time.Duration) {
	kq.cancelRetryLocked(key)
	kq.retryGen++
	gen := kq.retryGen
	// The scheduled function takes the queue lock, so it cannot run before
	// this method returns and the retry is recorded.
	kq.delayed[key] = &delayedRetry{
		app:   app,
		gen:   gen,
		timer: kq.newTimer(delay, func() { kq.requeueRetry(key, gen) }),
	}
	kq.reportRetryQueueLength()
}

func (kq *KeyedAppQueue) cancelRetryLocked(key string) {
	retry, ok := kq.delayed[key]
	if !ok {
		return
	}
	retry.timer.Stop()
	delete(kq.delayed, key)
	kq.reportRetryQueueLength()
}

// requeueRetry puts the delayed app back into the queue once its delay has
// passed, unless the retry was cancelled in the meantime.
func (kq *KeyedAppQueue) requeueRetry(key string, gen uint64) {
	kq.mu.Lock()
	defer kq.mu.Unlock()

	retry, ok := kq.delayed[key]
	if !ok || retry.gen != gen {
		// the retry was cancelled while its timer was already firing
		return
	}
	delete(kq.delayed, key)
	kq.reportRetryQueueLength()
	log.Debugf("retrying application %q", key)
	kq.enqueueLocked(retry.app)
}

func (kq *KeyedAppQueue) enqueueLocked(app appv1.Application) {
	key := AppKey(&app)

	// put into pending list if it is currently being processed
	if _, processing := kq.processing[key]; processing {
		kq.pending[key] = app
		kq.reportQueueLength()
		return
	}

	item := AppQueueItem{Key: key, Value: app}
	var newElem *list.Element
	if elem, exists := kq.keyMap[key]; exists {
		// put the new variant of same app at the same place in queue
		newElem = kq.items.InsertAfter(item, elem)
		kq.items.Remove(elem)
	} else {
		// add new app to the end of the queue
		newElem = kq.items.PushBack(item)
	}
	kq.keyMap[key] = newElem
	// signal workers they got job
	kq.notEmpty.Signal()
	kq.reportQueueLength()
}

// SetLengthGauge sets the gauge for tracking queue length
func (kq *KeyedAppQueue) SetLengthGauge(g gaugeSetter) {
	kq.mu.Lock()
	defer kq.mu.Unlock()
	kq.lengthGauge = g
	kq.reportQueueLength()
}

func (kq *KeyedAppQueue) reportQueueLength() {
	if kq.lengthGauge != nil {
		kq.lengthGauge.Set(float64(kq.items.Len() + len(kq.pending)))
	}
}

// reportRetryQueueLength reports the applications waiting for their
// retry delay.
func (kq *KeyedAppQueue) reportRetryQueueLength() {
	if kq.metrics != nil {
		kq.metrics.SetRetryQueueLength(len(kq.delayed))
	}
}

// DequeueWait blocks until an item is available
func (kq *KeyedAppQueue) DequeueWait(ctx context.Context) (*AppQueueItem, bool) {
	kq.mu.Lock()
	defer kq.mu.Unlock()

	// Wait until queue has items or context is done
	for kq.items.Len() == 0 {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, false
		default:
		}
		// Wait for signal (releases lock while waiting)
		kq.notEmpty.Wait()
	}

	elem := kq.items.Front()
	item := elem.Value.(AppQueueItem)
	kq.items.Remove(elem)
	delete(kq.keyMap, item.Key)
	// put into processing set, so the same app won't
	// be processed by different gouroutines in parallel
	kq.processing[item.Key] = struct{}{}
	kq.reportQueueLength()
	return &item, true
}

// Done removes the key from the processing set. If any updates for the app
// were received while processing the app, the last one is requeued.
func (kq *KeyedAppQueue) Done(key string) {
	kq.DoneWithError(key, appv1.Application{}, nil)
}

// DoneWithError removes the key from the processing set and reports the result
// of handling the app. If any updates for the app were received while
// processing it, the last one is requeued. Otherwise, when handling the app
// failed with a retriable error, the app is requeued after a growing delay.
func (kq *KeyedAppQueue) DoneWithError(key string, app appv1.Application, err error) {
	kq.mu.Lock()
	defer kq.mu.Unlock()

	if _, processing := kq.processing[key]; !processing {
		log.Errorf("Done called for app %q which is not being processed. this cannot happen!", key)
		return
	}
	delete(kq.processing, key)

	if pendingApp, ok := kq.pending[key]; ok {
		delete(kq.pending, key)
		if !sameVersion(&pendingApp, &app) {
			// New app version received while retrying
			kq.resetBudgetLocked(key, &pendingApp)
			kq.enqueueLocked(pendingApp)
			return
		}
	}

	if err == nil || !service.IsRetryable(err) {
		delete(kq.budgets, key)
		return
	}

	attempt := kq.budgets[key].attempts + 1
	logCtx := log.WithFields(log.Fields{
		"application": key,
		"attempt":     attempt,
		"maxAttempts": kq.retry.MaxAttempts,
	})
	namespace, name := splitAppKey(key)
	if !kq.retry.Enabled() || attempt > kq.retry.MaxAttempts {
		delete(kq.budgets, key)
		if kq.metrics != nil && kq.retry.Enabled() {
			kq.metrics.IncAppRetryExhausted(namespace, name)
		}
		logCtx.WithError(err).Error("failed to handle the application, giving up")
		return
	}
	delay := kq.retry.Backoff(attempt)
	kq.budgets[key] = retryBudget{attempts: attempt, resourceVersion: app.ResourceVersion}
	kq.scheduleRetryLocked(key, app, delay)
	if kq.metrics != nil {
		kq.metrics.IncAppRetry(namespace, name)
	}
	logCtx.WithError(err).Warnf("failed to handle the application, retrying in %v", delay)
}

type WorkerPool struct {
	queue   *KeyedAppQueue
	workers int
	jobChan chan *AppQueueItem
	ctx     context.Context
	cancel  context.CancelFunc
}

func (wp *WorkerPool) dequeueLoop() {
	for {
		select {
		case <-wp.ctx.Done():
			close(wp.jobChan)
			return
		default:
		}

		item, ok := wp.queue.DequeueWait(wp.ctx)
		if !ok {
			continue
		}

		select {
		case wp.jobChan <- item:
		case <-wp.ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) Stop() {
	wp.cancel()
}

// AppHandler handles a single application event. The error it returns decides
// whether the application is retried.
type AppHandler func(ctx context.Context, a appv1.Application) error

func NewWorkerPool(q *KeyedAppQueue, numWorkers int, f AppHandler) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		queue:   q,
		workers: numWorkers,
		jobChan: make(chan *AppQueueItem, numWorkers*2),
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		go func() {
			for job := range wp.jobChan {
				log.Debugf("Worker %d: handling app  %q", i, job.Key)
				err := f(ctx, job.Value)
				wp.queue.DoneWithError(job.Key, job.Value, err)
			}
		}()
	}
	go wp.dequeueLoop()
	return wp
}
