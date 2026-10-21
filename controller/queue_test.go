package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/mock"
	"github.com/tkrop/go-testing/test"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/util/workqueue"

	"github.com/tkrop/go-kube/controller"
)

// TODO: this is an AI generated test that needs to be reviewed and improved.

type queueOp struct {
	op    string
	item  string
	delay time.Duration
	check func(test.Test, controller.Queue[string], error)
}

type resultOp struct {
	item     string
	shutdown bool
}

func setupQueueOps(recorder *MockRecorder, ops []queueOp) {
	for _, op := range ops {
		switch op.op {
		case "add", "addafter":
			recorder.EXPECT().AddEvent(
				gomock.Any(), "test-queue", false).Times(1)
		case "requeue":
			recorder.EXPECT().AddEvent(
				gomock.Any(), "test-queue", true).Times(1)
		case "get":
			if op.item != "" {
				recorder.EXPECT().GetEvent(
					gomock.Any(), "test-queue", gomock.Any()).Times(1)
			}
		}
	}
}

func runQueueOps(
	t test.Test, queue controller.Queue[string],
	ops []queueOp, check func(test.Test, queueOp, resultOp),
) {
	ctx := context.Background()

	for _, op := range ops {
		var result resultOp

		switch op.op {
		case "add":
			queue.Add(ctx, op.item)
		case "addafter":
			queue.AddAfter(ctx, op.item, op.delay)
		case "get":
			result.item, result.shutdown = queue.Get(ctx)
			if op.check != nil {
				op.check(t, queue, nil)
			}
		case "done":
			queue.Done(ctx, op.item)
		case "requeue":
			err := queue.Requeue(ctx, op.item)
			if op.check != nil {
				op.check(t, queue, err)
			}
		case "forget":
			queue.Forget(ctx, op.item)
		case "shutdown":
			queue.ShutDown(ctx)
		case "len":
			if op.check != nil {
				op.check(t, queue, nil)
			}
		}

		check(t, op, result)
	}
}

type newDefaultQueueParams struct {
	name     string
	retries  int
	recorder controller.Recorder
}

var newDefaultQueueTestCases = map[string]newDefaultQueueParams{
	"without-recorder": {
		name:     "test-queue",
		retries:  3,
		recorder: nil,
	},
	"with-recorder": {
		name:    "monitored-queue",
		retries: 5,
	},
}

func TestNewDefaultQueue(t *testing.T) {
	test.Map(t, newDefaultQueueTestCases).
		Run(func(t test.Test, param newDefaultQueueParams) {
			// Given
			mocks := mock.NewMocks(t)
			var recorder controller.Recorder
			if param.recorder != nil {
				recorder = mock.Get(mocks, NewMockRecorder)
			}

			// When
			queue := controller.NewDefaultQueue(
				param.name, param.retries, recorder)

			// Then
			assert.NotNil(t, queue)
			assert.Equal(t, param.name, queue.Name())
		})
}

func TestNewRateLimitedQueue(t *testing.T) {
	test.Map(t, newDefaultQueueTestCases).
		Run(func(t test.Test, param newDefaultQueueParams) {
			// Given
			mocks := mock.NewMocks(t)
			var recorder controller.Recorder
			if param.recorder != nil {
				recorder = mock.Get(mocks, NewMockRecorder)
			}

			// When
			queue := controller.NewRateLimitedQueue(param.name,
				workqueue.DefaultTypedControllerRateLimiter[string](),
				param.retries, recorder)

			// Then
			assert.NotNil(t, queue)
			assert.Equal(t, param.name, queue.Name())
		})
}

type retryQueueRetriesParams struct {
	retries  int
	requeues int
	expect   []error
}

var retryQueueRetriesTestCases = map[string]retryQueueRetriesParams{
	"retries-none": {
		retries:  0,
		requeues: 2,
		expect:   []error{controller.ErrMaxRetries, controller.ErrMaxRetries},
	},

	// Exhausting the retries forgets the item and thereby resets the budget.
	"retries-once": {
		retries:  1,
		requeues: 3,
		expect:   []error{nil, controller.ErrMaxRetries, nil},
	},

	"retries-limited": {
		retries:  3,
		requeues: 4,
		expect:   []error{nil, nil, nil, controller.ErrMaxRetries},
	},

	"retries-infinite": {
		retries:  -1,
		requeues: 5,
		expect:   []error{nil, nil, nil, nil, nil},
	},
}

func TestRetryQueueRetries(t *testing.T) {
	test.Map(t, retryQueueRetriesTestCases).
		Run(func(t test.Test, param retryQueueRetriesParams) {
			// Given
			queue := controller.NewRetryQueue("test-queue",
				workqueue.NewTypedRateLimitingQueue(
					workqueue.DefaultTypedControllerRateLimiter[string]()),
				param.retries)
			ctx := context.Background()
			defer queue.ShutDown(ctx)

			// When
			errs := make([]error, 0, param.requeues)
			for range param.requeues {
				errs = append(errs, queue.Requeue(ctx, "item1"))
			}

			// Then
			assert.Equal(t, param.expect, errs)
		})
}

type retryQueueForgetParams struct {
	retries  int
	requeues int
	expect   error
}

var retryQueueForgetTestCases = map[string]retryQueueForgetParams{
	"forget-resets-budget": {
		retries:  1,
		requeues: 1,
		expect:   nil,
	},

	"forget-resets-exhausted-budget": {
		retries:  1,
		requeues: 3,
		expect:   nil,
	},
}

func TestRetryQueueForget(t *testing.T) {
	test.Map(t, retryQueueForgetTestCases).
		Run(func(t test.Test, param retryQueueForgetParams) {
			// Given
			queue := controller.NewRetryQueue("test-queue",
				workqueue.NewTypedRateLimitingQueue(
					workqueue.DefaultTypedControllerRateLimiter[string]()),
				param.retries)
			ctx := context.Background()
			defer queue.ShutDown(ctx)
			for range param.requeues {
				_ = queue.Requeue(ctx, "item1")
			}

			// When
			queue.Forget(ctx, "item1")

			// Then
			assert.Equal(t, param.expect, queue.Requeue(ctx, "item1"))
		})
}

type retryQueueParams struct {
	// setup mock.SetupFunc
	ops []queueOp
}

var retryQueueTestCases = map[string]retryQueueParams{
	"add-get-done": {
		ops: []queueOp{
			{op: "add", item: "item1"},
			{op: "len", check: func(
				t test.Test, q controller.Queue[string], _ error,
			) {
				assert.Equal(t, 1, q.Len(context.Background()))
			}},
			{
				op:   "get",
				item: "item1",
				check: func(
					_ test.Test, _ controller.Queue[string], _ error,
				) {
				},
			},
			{op: "done", item: "item1"},
		},
	},
	"requeue-success": {
		ops: []queueOp{
			{op: "add", item: "item1"},
			{op: "get", item: "item1"},
			{
				op:   "requeue",
				item: "item1",
				check: func(
					t test.Test, _ controller.Queue[string], err error,
				) {
					assert.Nil(t, err)
				},
			},
			{op: "done", item: "item1"},
		},
	},
	"requeue-max-retries": {
		ops: []queueOp{
			{op: "add", item: "item1"},
			{op: "get", item: "item1"},
			{op: "requeue", item: "item1"},
			{op: "done", item: "item1"},
			{op: "get", item: "item1"},
			{op: "requeue", item: "item1"},
			{op: "done", item: "item1"},
			{op: "get", item: "item1"},
			{op: "requeue", item: "item1"},
			{op: "done", item: "item1"},
			{op: "get", item: "item1"},
			{
				op:   "requeue",
				item: "item1",
				check: func(
					t test.Test, _ controller.Queue[string], err error,
				) {
					assert.Equal(t, controller.ErrMaxRetries, err)
				},
			},
			{op: "done", item: "item1"},
		},
	},
	"shutdown": {
		ops: []queueOp{
			{op: "add", item: "item1"},
			{op: "shutdown"},
			{op: "get", check: func(
				_ test.Test, _ controller.Queue[string], _ error,
			) {
			}},
		},
	},

	"add-after": {
		ops: []queueOp{
			{op: "addafter", item: "item1", delay: time.Millisecond},
			{op: "get", item: "item1"},
			{op: "done", item: "item1"},
		},
	},

	"add-after-zero": {
		ops: []queueOp{
			{op: "addafter", item: "item1"},
			{op: "len", check: func(
				t test.Test, q controller.Queue[string], _ error,
			) {
				assert.Equal(t, 1, q.Len(context.Background()))
			}},
			{op: "get", item: "item1"},
			{op: "done", item: "item1"},
		},
	},

	// The delaying queue keeps the earliest deadline of an item, so repeated
	// calls for a hot item coalesce into a single reconcile.
	"add-after-coalesced": {
		ops: []queueOp{
			{op: "addafter", item: "item1", delay: 50 * time.Millisecond},
			{op: "addafter", item: "item1", delay: 100 * time.Millisecond},
			{op: "addafter", item: "item1", delay: 200 * time.Millisecond},
			{op: "get", item: "item1"},
			{op: "done", item: "item1"},
			{op: "len", check: func(
				t test.Test, q controller.Queue[string], _ error,
			) {
				assert.Equal(t, 0, q.Len(context.Background()))
			}},
		},
	},
}

func TestRetryQueue(t *testing.T) {
	test.Map(t, retryQueueTestCases).
		Run(func(t test.Test, param retryQueueParams) {
			// Given
			queue := controller.NewRetryQueue("test-queue",
				workqueue.NewTypedRateLimitingQueue(
					workqueue.DefaultTypedControllerRateLimiter[string]()), 3)

			// When/Then
			runQueueOps(t, queue, param.ops,
				func(t test.Test, op queueOp, result resultOp) {
					if op.op == "get" && op.item != "" {
						assert.Equal(t, op.item, result.item)
						assert.False(t, result.shutdown)
					}
				})
		})
}

type monitorQueueParams struct {
	ops []queueOp
}

var monitorQueueTestCases = map[string]monitorQueueParams{
	"add-get": {
		ops: []queueOp{
			{op: "add", item: "item1"},
			{op: "get", item: "item1"},
			{op: "done", item: "item1"},
		},
	},

	"requeue": {
		ops: []queueOp{
			{op: "add", item: "item1"},
			{op: "get", item: "item1"},
			{op: "requeue", item: "item1"},
			{op: "done", item: "item1"},
		},
	},

	"requeue-then-shutdown": {
		ops: []queueOp{
			{op: "add", item: "item1"},
			{op: "get", item: "item1"},
			{op: "requeue", item: "item1"},
			{op: "shutdown"},
		},
	},

	"shutdown-get": {
		ops: []queueOp{
			{op: "shutdown"},
			{op: "get", item: ""},
		},
	},

	"forget": {
		ops: []queueOp{
			{op: "add", item: "item1"},
			{op: "get", item: "item1"},
			{op: "forget", item: "item1"},
			{op: "done", item: "item1"},
		},
	},

	"add-after": {
		ops: []queueOp{
			{op: "addafter", item: "item1", delay: time.Millisecond},
			{op: "get", item: "item1"},
			{op: "done", item: "item1"},
		},
	},

	// The queue time is measured from the first enqueuing, so the coalesced
	// calls only report a single get event.
	"add-after-coalesced": {
		ops: []queueOp{
			{op: "addafter", item: "item1", delay: 50 * time.Millisecond},
			{op: "addafter", item: "item1", delay: 100 * time.Millisecond},
			{op: "get", item: "item1"},
			{op: "done", item: "item1"},
		},
	},
}

func TestMonitorQueue(t *testing.T) {
	test.Map(t, monitorQueueTestCases).
		Run(func(t test.Test, param monitorQueueParams) {
			// Given
			mocks := mock.NewMocks(t)
			recorder := mock.Get(mocks, NewMockRecorder)
			baseQueue := controller.NewRetryQueue("test-queue",
				workqueue.NewTypedRateLimitingQueue(
					workqueue.DefaultTypedControllerRateLimiter[string]()), 3)

			// Expect RegisterLen call from NewMonitorQueue
			recorder.EXPECT().RegisterLen("test-queue", gomock.Any()).
				Times(1)

			// Expect recorder calls
			setupQueueOps(recorder, param.ops)

			queue := controller.NewMonitorQueue(baseQueue, recorder)

			// When/Then
			runQueueOps(t, queue, param.ops,
				func(t test.Test, op queueOp, result resultOp) {
					if op.op == "get" {
						if op.item != "" {
							assert.Equal(t, op.item, result.item)
							assert.False(t, result.shutdown)
						} else {
							assert.True(t, result.shutdown)
						}
					}
				})
		})
}
