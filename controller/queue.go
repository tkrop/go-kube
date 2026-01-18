package controller

import (
	"context"
	"time"

	"k8s.io/client-go/util/workqueue"

	sync "github.com/zolstein/sync-map"

	"github.com/tkrop/go-kube/errors"
)

var (
	// ErrQueue is a generic queue error.
	ErrQueue = errors.New("queue")
	// ErrMaxRetries is returned when an item has reached the maximum number
	// of retries.
	ErrMaxRetries = ErrQueue.New("max retries")
)

// Queue is a generic interface that any queue should implement.
type Queue[T comparable] interface {
	// Name returns the name of the queue.
	Name() string
	// Len returns the size of the queue.
	Len(ctx context.Context) int
	// Add will add an item to the queue.
	Add(ctx context.Context, item T)
	// Requeue will add an item to the queue in a requeue mode. If will
	// return an error if an item has reached max requeue attempts.
	Requeue(ctx context.Context, item T) error
	// Get is returning the first item in the queue. If the last item has not
	// been finished, the request will block until the queue is notified.
	Get(ctx context.Context) (item T, shutdown bool)
	// Done marks the item being used as done.
	Done(ctx context.Context, item T)
	// ShutDown stops the queue from accepting new items.
	ShutDown(ctx context.Context)
}

// NewDefaultQueue creates a new default queue with given number of retries
// and given recorder for monitoring.
func NewDefaultQueue(
	name string, retries int, recorder Recorder,
) Queue[string] {
	queue := NewRetryQueue(name, workqueue.NewTypedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[string]()), retries)

	// Wrap with monitoring if recorder is provided.
	if recorder != nil {
		queue = NewMonitorQueue(queue, recorder)
	}

	return queue
}

// RetryQueue is a rate limiting queue implementation.
type RetryQueue[T comparable] struct {
	name    string
	queue   workqueue.TypedRateLimitingInterface[T]
	retries int
}

// NewRetryQueue creates a new rate limiting queue with the given work queue
// and max retries.
func NewRetryQueue[T comparable](
	name string, queue workqueue.TypedRateLimitingInterface[T], retries int,
) Queue[T] {
	return &RetryQueue[T]{
		name:    name,
		queue:   queue,
		retries: retries,
	}
}

// Name returns the name of the queue.
func (q *RetryQueue[T]) Name() string {
	return q.name
}

// Len returns the size of the queue.
func (q *RetryQueue[T]) Len(_ context.Context) int {
	return q.queue.Len()
}

// Add will add an item to the queue.
func (q *RetryQueue[T]) Add(_ context.Context, item T) {
	q.queue.Add(item)
}

// Requeue re-adds the given item to the queue if the max retries has not been
// reached yet.
func (q *RetryQueue[T]) Requeue(_ context.Context, item T) error {
	if q.queue.NumRequeues(item) < q.retries {
		q.queue.AddRateLimited(item)

		return nil
	}

	q.queue.Forget(item)

	return ErrMaxRetries
}

// Get returns the first item in the queue. If the last item has not been
// finished, the request will block.
func (q *RetryQueue[T]) Get(_ context.Context) (item T, shutdown bool) {
	return q.queue.Get()
}

// Done marks the item being used as done.
func (q *RetryQueue[T]) Done(_ context.Context, item T) {
	q.queue.Done(item)
}

// ShutDown stops the queue from accepting new items.
func (q *RetryQueue[T]) ShutDown(_ context.Context) {
	q.queue.ShutDown()
}

// MonitorQueue is a wrapper for a monitored queue.
type MonitorQueue[T comparable] struct {
	Queue[T]
	mrec Recorder
	time sync.Map[T, time.Time]
}

// NewMonitorQueue creates a new monitored queue wrapping the given queue.
func NewMonitorQueue[T comparable](
	queue Queue[T], mrec Recorder,
) Queue[T] {
	// Register func/callback based metrics.
	mrec.RegisterLen(queue.Name(), queue.Len)

	return &MonitorQueue[T]{
		Queue: queue,
		mrec:  mrec,
		time:  sync.Map[T, time.Time]{},
	}
}

// Add will add an item to the queue.
func (q *MonitorQueue[T]) Add(ctx context.Context, item T) {
	q.time.LoadOrStore(item, time.Now())
	q.mrec.AddEvent(ctx, q.Name(), false)
	q.Queue.Add(ctx, item)
}

// Requeue re-adds the given item to the queue in requeue mode.
func (q *MonitorQueue[T]) Requeue(ctx context.Context, item T) error {
	q.time.LoadOrStore(item, time.Now())
	q.mrec.AddEvent(ctx, q.Name(), true)

	//nolint:wrapcheck // common controller error used in queue.
	return q.Queue.Requeue(ctx, item)
}

// Get returns the first item in the queue. If the last item has not been
// finished, the request will block.
func (q *MonitorQueue[T]) Get(ctx context.Context) (T, bool) {
	// Here should get blocked, warning with the mutexes.
	item, shutdown := q.Queue.Get(ctx)
	if shutdown {
		return item, shutdown
	}

	if time, ok := q.time.LoadAndDelete(item); ok {
		q.mrec.GetEvent(ctx, q.Name(), time)
	}

	return item, shutdown
}
