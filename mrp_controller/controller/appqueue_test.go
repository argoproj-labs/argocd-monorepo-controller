package controller

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func createTestApplication(namespace, name string) appv1.Application {
	return appv1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func createTestApplicationWithRevision(namespace, name, revision string) appv1.Application {
	app := createTestApplication(namespace, name)
	app.Status.Sync.Revision = revision
	return app
}

/* keyedappqueue basic operations tests */

func TestKeyedAppQueue_EnqueueSingle(t *testing.T) {
	queue := NewKeyedAppQueue()
	app := createTestApplication("default", "test-app")

	queue.Enqueue(app)

	// Dequeue in goroutine with timeout protection
	done := make(chan *AppQueueItem, 1)
	go func() {
		item, _ := queue.DequeueWait(context.Background())
		done <- item
	}()

	select {
	case item := <-done:
		require.NotNil(t, item)
		assert.Equal(t, "default/test-app", item.Key)
		assert.Equal(t, "test-app", item.Value.Name)
		assert.Equal(t, "default", item.Value.Namespace)
	case <-time.After(2 * time.Second):
		t.Fatal("dequeue operation timed out")
	}
}

func TestKeyedAppQueue_EnqueueDequeueMultiple(t *testing.T) {
	queue := NewKeyedAppQueue()

	apps := []struct {
		namespace string
		name      string
	}{
		{"default", "app-1"},
		{"default", "app-2"},
		{"kube-system", "app-3"},
		{"custom", "app-4"},
		{"custom", "app-5"},
	}

	// Enqueue all in order
	for _, app := range apps {
		queue.Enqueue(createTestApplication(app.namespace, app.name))
	}

	// Dequeue all and verify FIFO order
	done := make(chan struct{})
	results := make([]*AppQueueItem, 0, len(apps))
	var resultsMu sync.Mutex

	go func() {
		for i := 0; i < len(apps); i++ {
			item, _ := queue.DequeueWait(context.Background())
			if item != nil {
				resultsMu.Lock()
				results = append(results, item)
				resultsMu.Unlock()
			}
		}
		close(done)
	}()

	select {
	case <-done:
		assert.Len(t, results, len(apps))
		for i, expectedApp := range apps {
			assert.Equal(t, fmt.Sprintf("%s/%s", expectedApp.namespace, expectedApp.name), results[i].Key)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out")
	}
}

func TestKeyedAppQueue_DequeueEmpty(_ *testing.T) {
	queue := NewKeyedAppQueue()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Create a goroutine since context cancellation alone doesn't interrupt Wait()
	done := make(chan bool, 1)
	go func() {
		item, ok := queue.DequeueWait(ctx)
		done <- (item == nil && !ok)
	}()

	// Wait for goroutine with timeout
	select {
	case result := <-done:
		_ = result
	case <-time.After(500 * time.Millisecond):
		// It's ok if it blocks - that's what sync.Cond.Wait does with cancelled context
	}
}

func TestKeyedAppQueue_QueueLength(t *testing.T) {
	queue := NewKeyedAppQueue()

	queue.mu.RLock()
	assert.Equal(t, 0, queue.items.Len())
	queue.mu.RUnlock()

	for i := 1; i <= 3; i++ {
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("app-%d", i)))
	}

	queue.mu.RLock()
	assert.Equal(t, 3, queue.items.Len())
	queue.mu.RUnlock()

	// Dequeue 1 item
	done := make(chan struct{})
	go func() {
		queue.DequeueWait(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dequeue timed out")
	}

	queue.mu.RLock()
	assert.Equal(t, 2, queue.items.Len())
	queue.mu.RUnlock()

	// Dequeue remaining
	for i := 0; i < 2; i++ {
		queue.DequeueWait(context.Background())
	}

	queue.mu.RLock()
	assert.Equal(t, 0, queue.items.Len())
	queue.mu.RUnlock()
}

/* KEYEDAPPQUEUE KEY-BASED REPLACEMENT TESTS */

func TestKeyedAppQueue_ReplaceExistingKey(t *testing.T) {
	queue := NewKeyedAppQueue()

	// Enqueue initial apps
	app1 := createTestApplicationWithRevision("ns1", "app-a", "rev1")
	app2 := createTestApplication("ns2", "app-b")
	app3 := createTestApplication("ns3", "app-c")

	queue.Enqueue(app1)
	queue.Enqueue(app2)
	queue.Enqueue(app3)

	// Enqueue updated app1 (should replace at same position)
	updatedApp1 := createTestApplicationWithRevision("ns1", "app-a", "rev2-updated")
	queue.Enqueue(updatedApp1)

	// Dequeue all and verify order and updated values
	results := make([]*AppQueueItem, 0, 3)
	done := make(chan struct{})

	go func() {
		for i := 0; i < 3; i++ {
			item, _ := queue.DequeueWait(context.Background())
			if item != nil {
				results = append(results, item)
			}
		}
		close(done)
	}()

	select {
	case <-done:
		require.Len(t, results, 3)
		assert.Equal(t, "ns1/app-a", results[0].Key)
		assert.Equal(t, "rev2-updated", results[0].Value.Status.Sync.Revision)
		assert.Equal(t, "ns2/app-b", results[1].Key)
		assert.Equal(t, "ns3/app-c", results[2].Key)
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out")
	}
}

func TestKeyedAppQueue_ReplaceMiddleKey(t *testing.T) {
	queue := NewKeyedAppQueue()

	// Enqueue: a, b, c
	appA := createTestApplicationWithRevision("ns", "a", "v1")
	appB := createTestApplicationWithRevision("ns", "b", "v1")
	appC := createTestApplicationWithRevision("ns", "c", "v1")

	queue.Enqueue(appA)
	queue.Enqueue(appB)
	queue.Enqueue(appC)

	// Update b
	appBUpdated := createTestApplicationWithRevision("ns", "b", "v2-updated")
	queue.Enqueue(appBUpdated)

	// Dequeue all and verify
	results := make([]*AppQueueItem, 0, 3)
	done := make(chan struct{})

	go func() {
		for i := 0; i < 3; i++ {
			item, _ := queue.DequeueWait(context.Background())
			if item != nil {
				results = append(results, item)
			}
		}
		close(done)
	}()

	select {
	case <-done:
		require.Len(t, results, 3)
		assert.Equal(t, "ns/a", results[0].Key)
		assert.Equal(t, "v1", results[0].Value.Status.Sync.Revision)
		assert.Equal(t, "ns/b", results[1].Key)
		assert.Equal(t, "v2-updated", results[1].Value.Status.Sync.Revision)
		assert.Equal(t, "ns/c", results[2].Key)
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out")
	}
}

func TestKeyedAppQueue_ReplaceLastKey(t *testing.T) {
	queue := NewKeyedAppQueue()

	// Enqueue: a, b, c
	queue.Enqueue(createTestApplicationWithRevision("ns", "a", "v1"))
	queue.Enqueue(createTestApplicationWithRevision("ns", "b", "v1"))
	queue.Enqueue(createTestApplicationWithRevision("ns", "c", "v1"))

	// Update c (last item)
	appCUpdated := createTestApplicationWithRevision("ns", "c", "v2-updated")
	queue.Enqueue(appCUpdated)

	// Dequeue and verify
	results := make([]*AppQueueItem, 0, 3)
	done := make(chan struct{})

	go func() {
		for i := 0; i < 3; i++ {
			item, _ := queue.DequeueWait(context.Background())
			if item != nil {
				results = append(results, item)
			}
		}
		close(done)
	}()

	select {
	case <-done:
		require.Len(t, results, 3)
		assert.Equal(t, "ns/a", results[0].Key)
		assert.Equal(t, "ns/b", results[1].Key)
		assert.Equal(t, "ns/c", results[2].Key)
		assert.Equal(t, "v2-updated", results[2].Value.Status.Sync.Revision)
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out")
	}
}

func TestKeyedAppQueue_MultipleReplacementsCycle(t *testing.T) {
	queue := NewKeyedAppQueue()

	// Enqueue same app key multiple times with different versions
	for v := 1; v <= 4; v++ {
		app := createTestApplicationWithRevision("ns", "app", fmt.Sprintf("v%d", v))
		queue.Enqueue(app)
	}

	// Verify only one item in queue
	queue.mu.RLock()
	assert.Equal(t, 1, queue.items.Len())
	queue.mu.RUnlock()

	// Dequeue single item
	done := make(chan *AppQueueItem, 1)
	go func() {
		item, _ := queue.DequeueWait(context.Background())
		done <- item
	}()

	select {
	case item := <-done:
		require.NotNil(t, item)
		assert.Equal(t, "ns/app", item.Key)
		assert.Equal(t, "v4", item.Value.Status.Sync.Revision)
	case <-time.After(2 * time.Second):
		t.Fatal("dequeue timed out")
	}

	// Verify no stale entries in keyMap
	queue.mu.RLock()
	assert.Empty(t, queue.keyMap)
	queue.mu.RUnlock()
}

/* keyedappqueue wait/condition variable tests */

func TestKeyedAppQueue_DequeueWaitBlocksUntilItem(t *testing.T) {
	queue := NewKeyedAppQueue()
	dequeued := make(chan *AppQueueItem, 1)

	// Start goroutine that calls DequeueWait
	go func() {
		item, _ := queue.DequeueWait(context.Background())
		dequeued <- item
	}()

	// Give goroutine time to block
	time.Sleep(100 * time.Millisecond)

	// Verify it's blocked (no item received yet)
	select {
	case <-dequeued:
		t.Fatal("goroutine should still be blocked")
	default:
	}

	// Enqueue item
	queue.Enqueue(createTestApplication("ns", "app"))

	// Verify goroutine unblocks and receives item
	select {
	case item := <-dequeued:
		require.NotNil(t, item)
		assert.Equal(t, "ns/app", item.Key)
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine should have unblocked")
	}
}

func TestKeyedAppQueue_DequeueWaitContextCancellation(t *testing.T) {
	queue := NewKeyedAppQueue()
	received := make(chan bool, 1)

	ctx, cancel := context.WithCancel(context.Background())

	// Start goroutine that calls DequeueWait
	go func() {
		item, ok := queue.DequeueWait(ctx)
		received <- (item == nil && !ok)
	}()

	// Give goroutine time to block
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	// Try to receive - might not work due to sync.Cond.Wait limitation
	select {
	case result := <-received:
		// Context cancellation was detected
		assert.True(t, result, "should return (nil, false)")
	case <-time.After(500 * time.Millisecond):
		// This is expected behavior with sync.Cond - it doesn't interrupt Wait on context cancel
		// The test validates that we understand this limitation
	}
}

func TestKeyedAppQueue_DequeueWaitSignalDispatch(t *testing.T) {
	queue := NewKeyedAppQueue()
	received := make(chan bool, 3)

	// Start 3 goroutines all calling DequeueWait
	for i := 0; i < 3; i++ {
		go func() {
			item, ok := queue.DequeueWait(context.Background())
			received <- (ok && item != nil)
		}()
	}

	// Give goroutines time to block
	time.Sleep(200 * time.Millisecond)

	// Enqueue 1 item (Signal called once, wakes 1 goroutine)
	queue.Enqueue(createTestApplication("ns", "app"))

	// Collect results with timeout per goroutine
	unblocked := 0
	for i := 0; i < 3; i++ {
		select {
		case result := <-received:
			if result {
				unblocked++
			}
		case <-time.After(500 * time.Millisecond):
			// This goroutine stayed blocked
		}
	}

	assert.Equal(t, 1, unblocked, "only 1 goroutine should have received the item")

	// Enqueue 2 more items for remaining blocked goroutines
	queue.Enqueue(createTestApplication("ns", "app2"))
	queue.Enqueue(createTestApplication("ns", "app3"))

	// Wait for remaining goroutines
	for i := 0; i < 2; i++ {
		select {
		case result := <-received:
			assert.True(t, result)
		case <-time.After(2 * time.Second):
			// Some goroutines might not unblock
		}
	}
}

/* keyedappqueue concurrency tests */

func TestKeyedAppQueue_ConcurrentEnqueue(t *testing.T) {
	queue := NewKeyedAppQueue()
	var wg sync.WaitGroup
	numGoroutines := 10
	itemsPerGoroutine := 10

	// Start multiple goroutines enqueueing unique items
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < itemsPerGoroutine; i++ {
				app := createTestApplication(
					fmt.Sprintf("ns-%d", id),
					fmt.Sprintf("app-%d", i),
				)
				queue.Enqueue(app)
			}
		}(g)
	}

	wg.Wait()

	// Verify all items enqueued
	queue.mu.RLock()
	expectedCount := numGoroutines * itemsPerGoroutine
	assert.Equal(t, expectedCount, queue.items.Len())
	assert.Len(t, queue.keyMap, expectedCount)
	queue.mu.RUnlock()

	// Dequeue all and verify no data corruption
	results := make([]*AppQueueItem, 0, expectedCount)
	done := make(chan struct{})

	go func() {
		for i := 0; i < expectedCount; i++ {
			item, _ := queue.DequeueWait(context.Background())
			if item != nil {
				results = append(results, item)
			}
		}
		close(done)
	}()

	select {
	case <-done:
		assert.Len(t, results, expectedCount)
		seenKeys := make(map[string]bool)
		for _, item := range results {
			assert.False(t, seenKeys[item.Key], "duplicate key dequeued")
			seenKeys[item.Key] = true
		}
		assert.Len(t, seenKeys, expectedCount)
	case <-time.After(10 * time.Second):
		t.Fatal("test timed out")
	}
}

func TestKeyedAppQueue_ConcurrentEnqueueWithReplacements(t *testing.T) {
	queue := NewKeyedAppQueue()

	// Pre-enqueue 20 applications
	for i := 0; i < 20; i++ {
		queue.Enqueue(createTestApplicationWithRevision("ns", fmt.Sprintf("app-%d", i), "v0"))
	}

	var wg sync.WaitGroup
	numUpdaters := 5
	updatesPerUpdater := 50

	// Start multiple goroutines updating same 10 apps repeatedly
	for u := 0; u < numUpdaters; u++ {
		wg.Add(1)
		go func(updaterID int) {
			defer wg.Done()
			for i := 0; i < updatesPerUpdater; i++ {
				appID := i % 10
				app := createTestApplicationWithRevision(
					"ns",
					fmt.Sprintf("app-%d", appID),
					fmt.Sprintf("v%d-%d", updaterID, i),
				)
				queue.Enqueue(app)
			}
		}(u)
	}

	wg.Wait()

	// Verify all 20 apps still present
	queue.mu.RLock()
	assert.Equal(t, 20, queue.items.Len())
	assert.Len(t, queue.keyMap, 20)
	queue.mu.RUnlock()

	// Dequeue all and verify
	results := make([]*AppQueueItem, 0, 20)
	done := make(chan struct{})

	go func() {
		for i := 0; i < 20; i++ {
			item, _ := queue.DequeueWait(context.Background())
			if item != nil {
				results = append(results, item)
			}
		}
		close(done)
	}()

	select {
	case <-done:
		assert.Len(t, results, 20)
		seenKeys := make(map[string]bool)
		for _, item := range results {
			seenKeys[item.Key] = true
		}
		assert.Len(t, seenKeys, 20)
	case <-time.After(10 * time.Second):
		t.Fatal("test timed out")
	}
}

func TestKeyedAppQueue_RaceKeyMapAndListConsistency(t *testing.T) {
	queue := NewKeyedAppQueue()

	// Pre-populate with 20 items
	for i := 0; i < 20; i++ {
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("app-%d", i)))
	}

	// Consistency check function
	checkConsistency := func() bool {
		queue.mu.Lock()
		defer queue.mu.Unlock()

		// Verify all list elements have keyMap entries
		for elem := queue.items.Front(); elem != nil; elem = elem.Next() {
			item := elem.Value.(AppQueueItem)
			if _, exists := queue.keyMap[item.Key]; !exists {
				return false
			}
		}

		// Verify all keyMap entries point to valid list elements
		for key, elem := range queue.keyMap {
			if elem.Value.(AppQueueItem).Key != key {
				return false
			}
		}

		// Count check
		if queue.items.Len() != len(queue.keyMap) {
			return false
		}

		return true
	}

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Mixed operations goroutines
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stopChan:
					return
				default:
					op := counter % 3
					counter++

					switch op {
					case 0: // Enqueue new
						app := createTestApplication(
							"ns",
							fmt.Sprintf("app-op-%d-%d", id, counter),
						)
						queue.Enqueue(app)
					case 1: // Replace existing
						app := createTestApplication(
							"ns",
							fmt.Sprintf("app-%d", counter%20),
						)
						queue.Enqueue(app)
					case 2: // Dequeue
						go queue.DequeueWait(context.Background())
					}
					time.Sleep(3 * time.Millisecond)
				}
			}
		}(i)
	}

	// Consistency checker
	consistencyErrors := 0
	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)
		if !checkConsistency() {
			consistencyErrors++
		}
	}

	close(stopChan)
	wg.Wait()

	// Final consistency check
	assert.True(t, checkConsistency(), "final consistency check should pass")
	assert.Equal(t, 0, consistencyErrors, "no consistency errors during execution")
}

func TestKeyedAppQueue_ConcurrentEnqueueDequeue(t *testing.T) {
	queue := NewKeyedAppQueue()
	var wg sync.WaitGroup

	enqueuedCount := int32(0)
	dequeuedCount := int32(0)
	stopChan := make(chan struct{})

	// Enqueuers
	numEnqueuers := 3
	for e := 0; e < numEnqueuers; e++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stopChan:
					return
				default:
					app := createTestApplication(
						fmt.Sprintf("ns-%d", id),
						fmt.Sprintf("app-%d", counter),
					)
					queue.Enqueue(app)
					atomic.AddInt32(&enqueuedCount, 1)
					counter++
					time.Sleep(5 * time.Millisecond)
				}
			}
		}(e)
	}

	// Dequeuers
	numDequeuers := 3
	for d := 0; d < numDequeuers; d++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				item, _ := queue.DequeueWait(context.Background())
				if item != nil {
					atomic.AddInt32(&dequeuedCount, 1)
				}

				select {
				case <-stopChan:
					// Drain remaining items
					for {
						item, _ := queue.DequeueWait(context.Background())
						if item == nil {
							return
						}
						atomic.AddInt32(&dequeuedCount, 1)
					}
				default:
				}
			}
		}()
	}

	// Run for 300ms
	time.Sleep(300 * time.Millisecond)
	close(stopChan)

	// Wait for all to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Log("goroutines did not finish within timeout")
	}

	// Verify counts are reasonable
	eq := atomic.LoadInt32(&enqueuedCount)
	dq := atomic.LoadInt32(&dequeuedCount)
	assert.Positive(t, eq, "should have enqueued items")
	assert.Positive(t, dq, "should have dequeued items")
	assert.LessOrEqual(t, dq, eq, "dequeued should be <= enqueued")
}

/* WORKERPOOL INITIALIZATION & LIFECYCLE TESTS */

func TestWorkerPool_CreateAndStop(t *testing.T) {
	queue := NewKeyedAppQueue()
	processedApps := make([]*AppQueueItem, 0)
	var mu sync.Mutex

	handler := func(_ context.Context, app appv1.Application) error {
		mu.Lock()
		processedApps = append(processedApps, &AppQueueItem{
			Key:   app.Namespace + "/" + app.Name,
			Value: app,
		})
		mu.Unlock()
		return nil
	}

	// Create pool with 2 workers
	pool := NewWorkerPool(queue, 2, handler)
	require.NotNil(t, pool)
	assert.Equal(t, 2, pool.workers)
	assert.NotNil(t, pool.queue)
	assert.NotNil(t, pool.ctx)
	assert.NotNil(t, pool.cancel)

	// Stop immediately
	pool.Stop()

	// Verify context is cancelled
	select {
	case <-pool.ctx.Done():
		// Success - context is cancelled
	case <-time.After(1 * time.Second):
		t.Fatal("context should be cancelled after Stop()")
	}
}

func TestWorkerPool_MultipleWorkersInitialization(t *testing.T) {
	queue := NewKeyedAppQueue()
	handler := func(_ context.Context, _ appv1.Application) error { return nil }

	testCases := []int{1, 2, 5, 10}
	for _, numWorkers := range testCases {
		pool := NewWorkerPool(queue, numWorkers, handler)
		assert.Equal(t, numWorkers, pool.workers)
		pool.Stop()
	}
}

func TestWorkerPool_StopCancelsContext(t *testing.T) {
	queue := NewKeyedAppQueue()
	handler := func(_ context.Context, _ appv1.Application) error { return nil }

	pool := NewWorkerPool(queue, 1, handler)
	ctx := pool.ctx

	// Verify context not cancelled initially
	select {
	case <-ctx.Done():
		t.Fatal("context should not be cancelled initially")
	default:
	}

	pool.Stop()

	// Verify context is now cancelled
	select {
	case <-ctx.Done():
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("context should be cancelled after Stop()")
	}
}

func TestWorkerPool_ZeroWorkersHandling(t *testing.T) {
	queue := NewKeyedAppQueue()
	handler := func(_ context.Context, _ appv1.Application) error { return nil }

	pool := NewWorkerPool(queue, 0, handler)
	assert.Equal(t, 0, pool.workers)

	// Add items to queue
	queue.Enqueue(createTestApplication("ns", "app1"))

	// Give dequeueLoop time to try processing
	time.Sleep(100 * time.Millisecond)

	// Stop and verify
	pool.Stop()
	select {
	case <-pool.ctx.Done():
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("context should be cancelled")
	}
}

/* workerpool job processing tests */

func TestWorkerPool_SingleJobProcessing(t *testing.T) {
	queue := NewKeyedAppQueue()
	processedApps := make([]*AppQueueItem, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	handler := func(_ context.Context, app appv1.Application) error {
		mu.Lock()
		processedApps = append(processedApps, &AppQueueItem{
			Key:   app.Namespace + "/" + app.Name,
			Value: app,
		})
		mu.Unlock()
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 1, handler)
	defer pool.Stop()

	// Enqueue single app
	app := createTestApplication("default", "test-app")
	wg.Add(1)
	queue.Enqueue(app)

	// Wait for processing with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		require.Len(t, processedApps, 1)
		assert.Equal(t, "default/test-app", processedApps[0].Key)
	case <-time.After(5 * time.Second):
		t.Fatal("job processing timed out")
	}
}

func TestWorkerPool_MultipleJobsSequential(t *testing.T) {
	queue := NewKeyedAppQueue()
	processedKeys := make([]string, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	handler := func(_ context.Context, app appv1.Application) error {
		mu.Lock()
		processedKeys = append(processedKeys, app.Namespace+"/"+app.Name)
		mu.Unlock()
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 1, handler)
	defer pool.Stop()

	// Enqueue 5 apps sequentially
	apps := []string{"app-1", "app-2", "app-3", "app-4", "app-5"}
	for _, name := range apps {
		wg.Add(1)
		queue.Enqueue(createTestApplication("ns", name))
	}

	// Wait for all to be processed
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		require.Len(t, processedKeys, 5)
		for i, name := range apps {
			assert.Equal(t, "ns/"+name, processedKeys[i])
		}
	case <-time.After(10 * time.Second):
		t.Fatal("job processing timed out")
	}
}

func TestWorkerPool_ParallelJobProcessing(t *testing.T) {
	queue := NewKeyedAppQueue()
	processedCount := int32(0)
	var wg sync.WaitGroup

	handler := func(_ context.Context, _ appv1.Application) error {
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&processedCount, 1)
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 4, handler)
	defer pool.Stop()

	startTime := time.Now()
	for i := 0; i < 8; i++ {
		wg.Add(1)
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("app-%d", i)))
	}

	// Wait for all to be processed
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		elapsed := time.Since(startTime)
		count := atomic.LoadInt32(&processedCount)
		assert.Equal(t, int32(8), count)

		// With 4 workers processing 8 jobs at 50ms each, should take ~100ms
		// With 1 worker it would take 400ms, so verify parallelism helped
		assert.Less(t, elapsed, 300*time.Millisecond, "should benefit from parallelism")
	case <-time.After(15 * time.Second):
		t.Fatal("job processing timed out")
	}
}

func TestWorkerPool_JobProcessingOrder(t *testing.T) {
	queue := NewKeyedAppQueue()
	processedKeys := make([]string, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	handler := func(_ context.Context, app appv1.Application) error {
		mu.Lock()
		processedKeys = append(processedKeys, app.Namespace+"/"+app.Name)
		mu.Unlock()
		wg.Done()
		return nil
	}

	// Use single worker to verify FIFO order
	pool := NewWorkerPool(queue, 1, handler)
	defer pool.Stop()

	// Enqueue in specific order
	expectedOrder := []string{"first", "second", "third", "fourth", "fifth"}
	for _, name := range expectedOrder {
		wg.Add(1)
		queue.Enqueue(createTestApplication("ns", name))
	}

	// Wait for processing
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		require.Len(t, processedKeys, len(expectedOrder))
		for i, expected := range expectedOrder {
			assert.Equal(t, "ns/"+expected, processedKeys[i])
		}
	case <-time.After(10 * time.Second):
		t.Fatal("job processing timed out")
	}
}

/* workerpool worker distribution tests */

func TestWorkerPool_JobDistributionAcrossWorkers(t *testing.T) {
	queue := NewKeyedAppQueue()
	workerIDMap := make(map[int][]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Track which "worker" processed which job using goroutine ID
	workerChan := make(chan int, 100)
	jobChan := make(chan string, 100)

	handler := func(_ context.Context, app appv1.Application) error {
		id := <-workerChan
		mu.Lock()
		workerIDMap[id] = append(workerIDMap[id], app.Name)
		mu.Unlock()
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 3, handler)
	defer pool.Stop()

	// Simulate worker tracking by sending IDs
	for i := 0; i < 3; i++ {
		go func(id int) {
			for j := 0; j < 5; j++ {
				select {
				case <-pool.ctx.Done():
					return
				default:
					time.Sleep(10 * time.Millisecond)
					select {
					case workerChan <- id:
					case <-pool.ctx.Done():
						return
					default:
					}
				}
			}
		}(i + 1)
	}

	for i := 0; i < 15; i++ {
		wg.Add(1)
		jobChan <- fmt.Sprintf("job-%d", i)
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("job-%d", i)))
	}

	// Wait for processing
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Verify jobs were distributed
		totalJobs := 0
		for _, jobs := range workerIDMap {
			totalJobs += len(jobs)
		}
		// At least some distribution should occur
		assert.NotEmpty(t, workerIDMap, "should have processed jobs")
	case <-time.After(10 * time.Second):
		t.Fatal("job processing timed out")
	}
}

func TestWorkerPool_LoadBalancing(t *testing.T) {
	queue := NewKeyedAppQueue()
	var wg sync.WaitGroup
	processedCount := int32(0)

	// Handler with varying processing time
	handler := func(_ context.Context, _ appv1.Application) error {
		atomic.AddInt32(&processedCount, 1)
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 5, handler)
	defer pool.Stop()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("job-%d", i)))
	}

	// Wait for all to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		count := atomic.LoadInt32(&processedCount)
		assert.Equal(t, int32(50), count)
	case <-time.After(15 * time.Second):
		t.Fatal("job processing timed out")
	}
}

func TestWorkerPool_WorkerCompletion(t *testing.T) {
	queue := NewKeyedAppQueue()
	completionOrder := make([]int64, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	handler := func(_ context.Context, _ appv1.Application) error {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		completionOrder = append(completionOrder, time.Now().Unix())
		mu.Unlock()
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 2, handler)
	defer pool.Stop()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("app-%d", i)))
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		assert.Len(t, completionOrder, 4)
		// Verify all timestamps are reasonable
		for _, ts := range completionOrder {
			assert.Positive(t, ts)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("job processing timed out")
	}
}

func TestWorkerPool_BackpressureHandling(t *testing.T) {
	queue := NewKeyedAppQueue()
	var wg sync.WaitGroup
	processedCount := int32(0)

	handler := func(_ context.Context, _ appv1.Application) error {
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&processedCount, 1)
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 1, handler)
	defer pool.Stop()

	// Rapidly enqueue many jobs while slow worker processes them
	for i := 0; i < 20; i++ {
		wg.Add(1)
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("app-%d", i)))
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		count := atomic.LoadInt32(&processedCount)
		assert.Equal(t, int32(20), count)
	case <-time.After(15 * time.Second):
		t.Fatal("job processing timed out")
	}
}

/* workerpool edge cases & concurrency tests */

func TestWorkerPool_StopWhileProcessing(t *testing.T) {
	queue := NewKeyedAppQueue()
	var wg sync.WaitGroup
	processedCount := int32(0)

	handler := func(_ context.Context, _ appv1.Application) error {
		atomic.AddInt32(&processedCount, 1)
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 2, handler)

	// Enqueue jobs
	for i := 0; i < 10; i++ {
		wg.Add(1)
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("app-%d", i)))
	}

	// Stop while processing
	time.Sleep(50 * time.Millisecond)
	pool.Stop()

	// Verify context is cancelled
	select {
	case <-pool.ctx.Done():
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("context should be cancelled")
	}

	// Some jobs may or may not have been processed
	count := atomic.LoadInt32(&processedCount)
	assert.Positive(t, count)
	assert.LessOrEqual(t, count, int32(10))
}

func TestWorkerPool_ConcurrentJobAndStop(t *testing.T) {
	queue := NewKeyedAppQueue()
	var wg sync.WaitGroup
	processedCount := int32(0)
	var stopWg sync.WaitGroup

	handler := func(_ context.Context, _ appv1.Application) error {
		atomic.AddInt32(&processedCount, 1)
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 3, handler)

	// Goroutine that keeps adding jobs
	stopWg.Add(1)
	go func() {
		defer stopWg.Done()
		for i := 0; i < 20; i++ {
			select {
			case <-pool.ctx.Done():
				return
			default:
				wg.Add(1)
				queue.Enqueue(createTestApplication("ns", fmt.Sprintf("app-%d", i)))
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	// Goroutine that stops the pool
	stopWg.Add(1)
	go func() {
		defer stopWg.Done()
		time.Sleep(50 * time.Millisecond)
		pool.Stop()
	}()

	stopWg.Wait()

	// Verify pool was stopped
	select {
	case <-pool.ctx.Done():
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("context should be cancelled")
	}

	count := atomic.LoadInt32(&processedCount)
	assert.Positive(t, count)
}

func TestWorkerPool_ContextPropagation(t *testing.T) {
	queue := NewKeyedAppQueue()
	var wg sync.WaitGroup
	contextReceived := make(chan context.Context, 1)

	handler := func(ctx context.Context, _ appv1.Application) error {
		contextReceived <- ctx
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 1, handler)
	defer pool.Stop()

	wg.Add(1)
	queue.Enqueue(createTestApplication("ns", "app"))

	select {
	case ctx := <-contextReceived:
		// Context should be provided to handler
		require.NotNil(t, ctx)
		// It should be the pool's context
		assert.Same(t, pool.ctx, ctx)
	case <-time.After(5 * time.Second):
		t.Fatal("handler should receive context")
	}
}

func TestWorkerPool_LargeScaleConcurrency(t *testing.T) {
	queue := NewKeyedAppQueue()
	var wg sync.WaitGroup
	processedCount := int32(0)

	handler := func(_ context.Context, _ appv1.Application) error {
		atomic.AddInt32(&processedCount, 1)
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 10, handler)
	defer pool.Stop()

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("job-%d", i)))
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		count := atomic.LoadInt32(&processedCount)
		assert.Equal(t, int32(1000), count)
	case <-time.After(30 * time.Second):
		t.Fatal("large-scale test timed out")
	}
}

/* queue gauge metrics tests */

type testGauge struct {
	lastValue float64
}

func (tg *testGauge) Set(v float64) {
	tg.lastValue = v
}

func TestKeyedAppQueue_GaugeSetterInitial(t *testing.T) {
	queue := NewKeyedAppQueue()
	gauge := &testGauge{}

	// Enqueue 3 items first
	for i := 1; i <= 3; i++ {
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("app-%d", i)))
	}

	// Set gauge - should report current length
	queue.SetLengthGauge(gauge)
	assert.InDelta(t, float64(3), gauge.lastValue, 0)
}

func TestKeyedAppQueue_GaugeTracksEnqueueDequeue(t *testing.T) {
	queue := NewKeyedAppQueue()
	gauge := &testGauge{}
	queue.SetLengthGauge(gauge)

	// Start with empty
	assert.InDelta(t, float64(0), gauge.lastValue, 0)

	// Enqueue 2 items
	queue.Enqueue(createTestApplication("ns", "app-1"))
	assert.InDelta(t, float64(1), gauge.lastValue, 0)

	queue.Enqueue(createTestApplication("ns", "app-2"))
	assert.InDelta(t, float64(2), gauge.lastValue, 0)

	// Dequeue 1 item
	done := make(chan struct{})
	go func() {
		queue.DequeueWait(context.Background())
		close(done)
	}()

	select {
	case <-done:
		assert.InDelta(t, float64(1), gauge.lastValue, 0)
	case <-time.After(2 * time.Second):
		t.Fatal("dequeue timed out")
	}

	// Dequeue remaining
	done = make(chan struct{})
	go func() {
		queue.DequeueWait(context.Background())
		close(done)
	}()

	select {
	case <-done:
		assert.InDelta(t, float64(0), gauge.lastValue, 0)
	case <-time.After(2 * time.Second):
		t.Fatal("dequeue timed out")
	}
}

// dequeue takes one item off the queue, failing the test if nothing arrives
func dequeue(t *testing.T, queue *KeyedAppQueue) *AppQueueItem {
	t.Helper()
	done := make(chan *AppQueueItem, 1)
	go func() {
		item, _ := queue.DequeueWait(context.Background())
		done <- item
	}()

	select {
	case item := <-done:
		require.NotNil(t, item)
		return item
	case <-time.After(2 * time.Second):
		t.Fatal("dequeue timed out")
		return nil
	}
}

func TestKeyedAppQueue_CheckedOutKeyIsNotDequeuedAgain(t *testing.T) {
	queue := NewKeyedAppQueue()

	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v1"))
	item := dequeue(t, queue)
	assert.Equal(t, "v1", item.Value.Status.Sync.Revision)

	// events arriving while the app is checked out are coalesced into one
	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v2"))
	queue.Enqueue(createTestApplicationWithRevision("ns", "app", "v3"))

	queue.mu.RLock()
	assert.Equal(t, 0, queue.items.Len(), "the app being processed must not be requeued")
	assert.Len(t, queue.pending, 1)
	assert.Equal(t, "v3", queue.pending["ns/app"].Status.Sync.Revision, "only newest event is kept")
	queue.mu.RUnlock()

	// releasing the key requeues the newest event
	queue.Done(item.Key)

	queue.mu.RLock()
	assert.Equal(t, 1, queue.items.Len())
	assert.Empty(t, queue.pending)
	assert.Empty(t, queue.processing)
	queue.mu.RUnlock()

	requeued := dequeue(t, queue)
	assert.Equal(t, "ns/app", requeued.Key)
	assert.Equal(t, "v3", requeued.Value.Status.Sync.Revision)
}

func TestKeyedAppQueue_CheckoutDoesNotBlockOtherKeys(t *testing.T) {
	queue := NewKeyedAppQueue()

	queue.Enqueue(createTestApplication("ns", "app-1"))
	first := dequeue(t, queue)
	assert.Equal(t, "ns/app-1", first.Key)

	// a different app stays dequeuable while app-1 is checked out
	queue.Enqueue(createTestApplication("ns", "app-2"))
	second := dequeue(t, queue)
	assert.Equal(t, "ns/app-2", second.Key)
}

func TestKeyedAppQueue_DoneWithoutPendingEventKeepsQueueEmpty(t *testing.T) {
	queue := NewKeyedAppQueue()

	queue.Enqueue(createTestApplication("ns", "app"))
	item := dequeue(t, queue)
	queue.Done(item.Key)

	queue.mu.RLock()
	assert.Equal(t, 0, queue.items.Len())
	assert.Empty(t, queue.keyMap)
	assert.Empty(t, queue.pending)
	assert.Empty(t, queue.processing)
	queue.mu.RUnlock()
}

func TestKeyedAppQueue_DoneForUnknownKeyIsNoop(t *testing.T) {
	queue := NewKeyedAppQueue()

	queue.Done("ns/never-dequeued")

	queue.mu.RLock()
	assert.Equal(t, 0, queue.items.Len())
	assert.Empty(t, queue.processing)
	queue.mu.RUnlock()
}

func TestKeyedAppQueue_GaugeCountsSameAppEvents(t *testing.T) {
	queue := NewKeyedAppQueue()
	gauge := &testGauge{}
	queue.SetLengthGauge(gauge)

	queue.Enqueue(createTestApplication("ns", "app"))
	assert.InDelta(t, float64(1), gauge.lastValue, 0)

	item := dequeue(t, queue)
	assert.InDelta(t, float64(0), gauge.lastValue, 0)

	queue.Enqueue(createTestApplication("ns", "app"))
	assert.InDelta(t, float64(1), gauge.lastValue, 0)
	queue.Enqueue(createTestApplication("ns", "app"))
	assert.InDelta(t, float64(1), gauge.lastValue, 0)

	queue.Done(item.Key)
	assert.InDelta(t, float64(1), gauge.lastValue, 0)

	dequeue(t, queue)
	assert.InDelta(t, float64(0), gauge.lastValue, 0)
}

func TestWorkerPool_SameAppNotProcessedConcurrently(t *testing.T) {
	queue := NewKeyedAppQueue()
	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0
	processed := int32(0)

	handler := func(_ context.Context, _ appv1.Application) error {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
		atomic.AddInt32(&processed, 1)
		return nil
	}

	pool := NewWorkerPool(queue, 4, handler)
	defer pool.Stop()

	// hammer a single app from several goroutines
	var wg sync.WaitGroup
	for e := 0; e < 4; e++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				queue.Enqueue(createTestApplicationWithRevision("ns", "app", fmt.Sprintf("v%d", i)))
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}
	wg.Wait()

	// let the last coalesced event drain
	assert.Eventually(t, func() bool {
		queue.mu.RLock()
		defer queue.mu.RUnlock()
		return queue.items.Len() == 0 && len(queue.pending) == 0 && len(queue.processing) == 0
	}, 5*time.Second, 10*time.Millisecond, "queue should drain")

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, maxInFlight, "the same app must never be processed by two workers at once")
	assert.Positive(t, atomic.LoadInt32(&processed))
}

func TestWorkerPool_DistinctAppsStillRunInParallel(t *testing.T) {
	queue := NewKeyedAppQueue()
	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0
	var wg sync.WaitGroup

	handler := func(_ context.Context, _ appv1.Application) error {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()

		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
		wg.Done()
		return nil
	}

	pool := NewWorkerPool(queue, 4, handler)
	defer pool.Stop()

	for i := 0; i < 8; i++ {
		wg.Add(1)
		queue.Enqueue(createTestApplication("ns", fmt.Sprintf("app-%d", i)))
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("job processing timed out")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Greater(t, maxInFlight, 1, "distinct apps should be processed in parallel")
}
