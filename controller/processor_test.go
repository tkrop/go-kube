package controller_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/mock"
	"github.com/tkrop/go-testing/test"

	"github.com/tkrop/go-kube/controller"
)

func pod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
	}
}

func CallQueueName(name string) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockQueue[string]).EXPECT().
			Name().Return(name)
	}
}

func CallQueueGet(key string, shutdown bool) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockQueue[string]).EXPECT().
			Get(gomock.Any()).Return(key, shutdown)
	}
}

func CallQueueAdd(key string) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockQueue[string]).EXPECT().
			Add(gomock.Any(), key)
	}
}

func CallQueueRequeue(key string, err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockQueue[string]).EXPECT().
			Requeue(gomock.Any(), key).Return(err)
	}
}

func CallQueueDone(key string) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockQueue[string]).EXPECT().
			Done(gomock.Any(), key)
	}
}

func CallQueueShutDown() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockQueue[string]).EXPECT().
			ShutDown(gomock.Any())
	}
}

func CallProcessorHandlerNotify(key string) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockHandler[*corev1.Pod]).EXPECT().
			Notify(gomock.Any(), key, gomock.Any())
	}
}

func CallRecorderDoneEvent(
	name string, success bool, start time.Time,
) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRecorder).EXPECT().
			DoneEvent(ctx, name, success, start)
	}
}

func CallRecorderDoneEventAny(
	name string, success bool,
) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRecorder).EXPECT().
			DoneEvent(ctx, name, success, gomock.Any())
	}
}

func CallHandlerHandle(
	obj runtime.Object, err error,
) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockHandler[*corev1.Pod]).EXPECT().
			Handle(gomock.Any(), obj).Return(err)
	}
}

func CallHandlerHandlePanic(
	obj runtime.Object, msg string,
) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockHandler[*corev1.Pod]).EXPECT().
			Handle(gomock.Any(), obj).
			Do(func(context.Context, runtime.Object) {
				panic(msg)
			})
	}
}

func CallIndexerGetByKey(
	key string, obj any, exists bool, err error,
) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockIndexer).EXPECT().
			GetByKey(key).Return(obj, exists, err)
	}
}

type newResourceEventHandlerParams struct {
	expectHandler bool
	expectQueue   bool
}

var newResourceEventHandlerTestCases = map[string]newResourceEventHandlerParams{
	"valid-handler-and-queue": {
		expectHandler: true,
		expectQueue:   true,
	},
}

// TODO: test is not really testing anything meaningful yet.
func TestNewResourceEventHandler(t *testing.T) {
	test.Map(t, newResourceEventHandlerTestCases).
		Run(func(t test.Test, _ newResourceEventHandlerParams) {
			// Given
			mocks := mock.NewMocks(t)
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			queue := mock.Get(mocks, NewMockQueue[string])

			// When
			result := controller.NewResourceEventHandler[*corev1.Pod](
				handler, queue)

			// Then
			assert.NotNil(t, result)
		})
}

type onAddParams struct {
	setup  mock.SetupFunc
	obj    any
	isInit bool
}

var onAddTestCases = map[string]onAddParams{
	"valid-object": {
		setup:  mock.Chain(CallQueueAdd("default/test-pod")),
		obj:    pod("test-pod"),
		isInit: false,
	},

	"invalid-object-no-meta": {
		setup:  mock.Chain(CallProcessorHandlerNotify("")),
		obj:    "invalid",
		isInit: false,
	},

	"valid-object-initial-add": {
		setup:  mock.Chain(CallQueueAdd("default/init-pod")),
		obj:    pod("init-pod"),
		isInit: true,
	},
}

func TestOnAdd(t *testing.T) {
	test.Map(t, onAddTestCases).
		Run(func(t test.Test, param onAddParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			queue := mock.Get(mocks, NewMockQueue[string])
			eventHandler := controller.NewResourceEventHandler[*corev1.Pod](
				handler, queue)

			// When
			eventHandler.OnAdd(param.obj, param.isInit)
		})
}

type onUpdateParams struct {
	setup  mock.SetupFunc
	oldObj any
	newObj any
}

var onUpdateTestCases = map[string]onUpdateParams{
	"valid-update": {
		setup:  mock.Chain(CallQueueAdd("default/updated-pod")),
		oldObj: pod("updated-pod"),
		newObj: pod("updated-pod"),
	},

	"invalid-new-object": {
		setup:  mock.Chain(CallProcessorHandlerNotify("")),
		oldObj: pod("old-pod"),
		newObj: "invalid",
	},
}

func TestOnUpdate(t *testing.T) {
	test.Map(t, onUpdateTestCases).
		Run(func(t test.Test, param onUpdateParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			queue := mock.Get(mocks, NewMockQueue[string])
			eventHandler := controller.NewResourceEventHandler[*corev1.Pod](
				handler, queue)

			// When
			eventHandler.OnUpdate(param.oldObj, param.newObj)
		})
}

type onDeleteParams struct {
	setup mock.SetupFunc
	obj   any
}

var onDeleteTestCases = map[string]onDeleteParams{
	"valid-delete": {
		setup: mock.Chain(CallQueueAdd("default/deleted-pod")),
		obj:   pod("deleted-pod"),
	},

	"delete-with-tombstone": {
		setup: mock.Chain(CallQueueAdd("default/tombstone-pod")),
		obj: cache.DeletedFinalStateUnknown{
			Key: "default/tombstone-pod",
			Obj: pod("tombstone-pod"),
		},
	},

	"invalid-delete-object": {
		setup: mock.Chain(CallProcessorHandlerNotify("")),
		obj:   "invalid",
	},
}

func TestOnDelete(t *testing.T) {
	test.Map(t, onDeleteTestCases).
		Run(func(t test.Test, param onDeleteParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			queue := mock.Get(mocks, NewMockQueue[string])
			eventHandler := controller.NewResourceEventHandler[*corev1.Pod](
				handler, queue)

			// When
			eventHandler.OnDelete(param.obj)
		})
}

type newProcessorParams struct {
	workers int
}

var newProcessorTestCases = map[string]newProcessorParams{
	"with-single-worker": {
		workers: 1,
	},

	"with-multiple-workers": {
		workers: 3,
	},

	"with-zero-workers": {
		workers: 0,
	},
}

// TODO: test is not really testing anything meaningful yet.
func TestNewProcessor(t *testing.T) {
	test.Map(t, newProcessorTestCases).
		Run(func(t test.Test, param newProcessorParams) {
			// Given
			mocks := mock.NewMocks(t)
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{}, &corev1.Pod{}, 0, cache.Indexers{})
			queue := mock.Get(mocks, NewMockQueue[string])
			recorder := mock.Get(mocks, NewMockRecorder)

			// When
			processor := controller.NewProcessor[*corev1.Pod](
				handler, informer, param.workers, queue, recorder)

			// Then
			assert.NotNil(t, processor)
		})
}

type runParams struct {
	setup   mock.SetupFunc
	workers int
}

var runTestCases = map[string]runParams{
	"single-worker": {
		setup: mock.Chain(
			CallProcessorHandlerNotify("starting processor"),
			CallQueueGet("worker-1", true),
			CallProcessorHandlerNotify("stopping processor"),
			CallQueueShutDown(),
		),
		workers: 1,
	},

	"multiple-workers": {
		setup: mock.Chain(
			CallProcessorHandlerNotify("starting processor"),
			CallQueueGet("worker-1", true),
			CallQueueGet("worker-2", true),
			CallQueueGet("worker-3", true),
			CallProcessorHandlerNotify("stopping processor"),
			CallQueueShutDown(),
		),
		workers: 3,
	},
}

func TestRun(t *testing.T) {
	test.Map(t, runTestCases).
		Run(func(t test.Test, param runParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			informer := mock.Get(mocks, NewMockSharedIndexInformer)
			indexer := mock.Get(mocks, NewMockIndexer)
			queue := mock.Get(mocks, NewMockQueue[string])
			recorder := mock.Get(mocks, NewMockRecorder)

			informer.EXPECT().GetIndexer().Return(indexer).AnyTimes()

			processor := controller.NewProcessor[*corev1.Pod](
				handler, informer, param.workers, queue, recorder)

			ctx, cancel := context.WithTimeout(
				context.Background(), 100*time.Millisecond)
			defer cancel()

			// When
			processor.Run(ctx)
		})
}

type processParams struct {
	setup        mock.SetupFunc
	withRecorder bool
	withQueue    bool
}

var processTestCases = map[string]processParams{
	"exit-on-shutdown": {
		setup: mock.Chain(
			CallQueueGet("", true)),
		withQueue: true,
	},

	"success-with-recorder": {
		setup: mock.Chain(
			CallQueueGet("default/test-pod", false),
			CallIndexerGetByKey("default/test-pod", pod("test-pod"), true, nil),
			CallHandlerHandle(pod("test-pod"), nil),
			CallQueueName("test-queue"),
			CallRecorderDoneEventAny("test-queue", true),
			CallQueueDone("default/test-pod"),
			CallQueueGet("", true)),
		withRecorder: true,
		withQueue:    true,
	},

	"success-without-recorder": {
		setup: mock.Chain(
			CallQueueGet("default/test-pod", false),
			CallIndexerGetByKey("default/test-pod", pod("test-pod"), true, nil),
			CallHandlerHandle(pod("test-pod"), nil),
			CallQueueDone("default/test-pod"),
			CallQueueGet("", true)),
		withQueue: true,
	},

	"indexer-get-error": {
		setup: mock.Chain(
			CallQueueGet("default/error-pod", false),
			CallIndexerGetByKey("default/error-pod", nil, false,
				assert.AnError),
			CallProcessorHandlerNotify("default/error-pod"),
			CallQueueDone("default/error-pod"),
			CallQueueGet("", true)),
		withQueue: true,
	},

	"object-not-exists": {
		setup: mock.Chain(
			CallQueueGet("default/missing-pod", false),
			CallIndexerGetByKey("default/missing-pod", nil, false, nil),
			CallQueueDone("default/missing-pod"),
			CallQueueGet("", true)),
		withQueue: true,
	},

	"type-assertion-error": {
		setup: mock.Chain(
			CallQueueGet("default/invalid-type", false),
			CallIndexerGetByKey("default/invalid-type",
				"not-a-runtime-object", true, nil),
			CallProcessorHandlerNotify("default/invalid-type"),
			CallQueueDone("default/invalid-type"),
			CallQueueGet("", true)),
		withQueue: true,
	},

	"handler-error-with-requeue": {
		setup: mock.Chain(
			CallQueueGet("default/handler-error", false),
			CallIndexerGetByKey("default/handler-error",
				pod("handler-error"), true, nil),
			CallHandlerHandle(pod("handler-error"), assert.AnError),
			CallQueueRequeue("default/handler-error", nil),
			CallQueueName("test-queue"),
			CallRecorderDoneEventAny("test-queue", false),
			CallQueueDone("default/handler-error"),
			CallQueueGet("", true)),
		withRecorder: true,
		withQueue:    true,
	},

	"handler-error-with-requeue-failure": {
		setup: mock.Chain(
			CallQueueGet("default/requeue-error", false),
			CallIndexerGetByKey("default/requeue-error",
				pod("requeue-error"), true, nil),
			CallHandlerHandle(pod("requeue-error"), assert.AnError),
			CallQueueRequeue("default/requeue-error", assert.AnError),
			CallProcessorHandlerNotify("default/requeue-error"),
			CallQueueName("test-queue"),
			CallRecorderDoneEventAny("test-queue", false),
			CallQueueDone("default/requeue-error"),
			CallQueueGet("", true)),
		withRecorder: true,
		withQueue:    true,
	},

	"handler-panic-recovery": {
		setup: mock.Chain(
			CallQueueGet("default/panic-pod", false),
			CallIndexerGetByKey("default/panic-pod", pod("panic-pod"),
				true, nil),
			CallHandlerHandlePanic(pod("panic-pod"), "handler panic"),
			CallProcessorHandlerNotify("default/panic-pod"),
			CallQueueDone("default/panic-pod"),
			CallQueueGet("", true)),
		withQueue: true,
	},
}

func TestProcess(t *testing.T) {
	test.Map(t, processTestCases).
		Run(func(t test.Test, param processParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			informer := mock.Get(mocks, NewMockSharedIndexInformer)
			indexer := mock.Get(mocks, NewMockIndexer)
			queue := mock.Get(mocks, NewMockQueue[string])
			recorder := mock.Get(mocks, NewMockRecorder)

			informer.EXPECT().GetIndexer().Return(indexer).AnyTimes()

			var processor *controller.Processor[*corev1.Pod]
			if param.withRecorder {
				processor = controller.NewProcessor[*corev1.Pod](
					handler, informer, 1, queue, recorder)
			} else if param.withQueue {
				processor = controller.NewProcessor[*corev1.Pod](
					handler, informer, 1, queue, nil)
			} else {
				processor = controller.NewProcessor[*corev1.Pod](
					handler, informer, 1, nil, nil)
			}

			// When
			processor.Process(ctx)
		})
}
