package controller_test

import (
	"context"
	"testing"

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
	check func(test.Test, controller.Queue[string], error)
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
}

//nolint:gocognit // TODO: improve test design.
//revive:disable-next-line:cognitive-complexity // TODO: improve test design.
func TestRetryQueue(t *testing.T) {
	test.Map(t, retryQueueTestCases).
		Run(func(t test.Test, param retryQueueParams) {
			// Given
			queue := controller.NewRetryQueue("test-queue",
				workqueue.NewTypedRateLimitingQueue(
					workqueue.DefaultTypedControllerRateLimiter[string]()), 3)
			ctx := context.Background()

			// When/Then
			for _, op := range param.ops {
				switch op.op {
				case "add":
					queue.Add(ctx, op.item)
				case "get":
					item, shutdown := queue.Get(ctx)
					if op.check != nil {
						op.check(t, queue, nil)
					}
					if op.item != "" {
						assert.Equal(t, op.item, item)
						assert.False(t, shutdown)
					}
				case "done":
					queue.Done(ctx, op.item)
				case "requeue":
					err := queue.Requeue(ctx, op.item)
					if op.check != nil {
						op.check(t, queue, err)
					}
				case "shutdown":
					queue.ShutDown(ctx)
				case "len":
					if op.check != nil {
						op.check(t, queue, nil)
					}
				}
			}
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
			for _, op := range param.ops {
				switch op.op {
				case "add":
					recorder.EXPECT().AddEvent(
						gomock.Any(), "test-queue", false).Times(1)
				case "requeue":
					recorder.EXPECT().AddEvent(
						gomock.Any(), "test-queue", true).Times(1)
				case "get":
					recorder.EXPECT().GetEvent(
						gomock.Any(), "test-queue", gomock.Any()).Times(1)
				}
			}

			queue := controller.NewMonitorQueue(baseQueue, recorder)
			ctx := context.Background()

			// When/Then
			for _, op := range param.ops {
				switch op.op {
				case "add":
					queue.Add(ctx, op.item)
				case "get":
					item, shutdown := queue.Get(ctx)
					if op.item != "" {
						assert.Equal(t, op.item, item)
						assert.False(t, shutdown)
					} else {
						assert.True(t, shutdown)
					}
				case "done":
					queue.Done(ctx, op.item)
				case "requeue":
					_ = queue.Requeue(ctx, op.item)
				case "shutdown":
					queue.ShutDown(ctx)
				}
			}
		})
}
