package controller_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"go.uber.org/mock/gomock"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/mock"
	"github.com/tkrop/go-testing/reflect"
	"github.com/tkrop/go-testing/test"

	"github.com/tkrop/go-kube/controller"
	"github.com/tkrop/go-kube/errors"
)

// TODO: this is an AI generated test that needs to be reviewed and improved.

var (
	ctx = context.Background()
	d1  = &Object{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "dummy",
	}}
	p1 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "pod-no-owner",
	}}
	p2 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "pod-owner",
		OwnerReferences: []metav1.OwnerReference{{
			Name: "owner", UID: "owner-id",
		}},
	}}
	p3 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "pod-other-name",
		OwnerReferences: []metav1.OwnerReference{{
			Name: "other-owner", UID: "owner-id",
		}},
	}}
	p4 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "pod-other-id",
		OwnerReferences: []metav1.OwnerReference{{
			Name: "owner", UID: "other-id",
		}},
	}}
	p5 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "pod-other-both",
		OwnerReferences: []metav1.OwnerReference{{
			Name: "other-owner", UID: "other-id",
		}},
	}}
	p6 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "other", Name: "pod-no-owner",
	}}
	p7 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "other", Name: "pod-owner",
		OwnerReferences: []metav1.OwnerReference{{
			Name: "owner", UID: "owner-id",
		}},
	}}
)

// CallRecorderLen sets up expectations for metrics recorder.
func CallRecorderLen(name string) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRecorder).EXPECT().
			RegisterLen(name, gomock.Any()).Return()
	}
}

func CallRecorderAddEvent() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRecorder).EXPECT().
			AddEvent(gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()
	}
}

func CallRecorderEventAny() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		recorder := mock.Get(mocks, NewMockRecorder)
		recorder.EXPECT().
			GetEvent(gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()

		return recorder.EXPECT().DoneEvent(gomock.Any(), gomock.Any(),
			gomock.Any(), gomock.Any()).AnyTimes()
	}
}

func CallHandlerHandleAny() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockHandler[*corev1.Pod]).EXPECT().
			Handle(gomock.Any(), gomock.Any()).Return(nil).
			AnyTimes()
	}
}

func CallHandlerNotify(key string, err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockHandler[*corev1.Pod]).EXPECT().
			Notify(ctx, key, err).Return().Times(1)
	}
}

func CallHandlerNotifyAny() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockHandler[*corev1.Pod]).EXPECT().
			Notify(gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()
	}
}

type jitterParams struct {
	key    string
	window time.Duration
	expect time.Duration
}

var jitterTestCases = map[string]jitterParams{
	"zero-window": {
		key: "default/pod",
	},

	"negative-window": {
		key:    "default/pod",
		window: -time.Minute,
	},

	"single-nanosecond-window": {
		key:    "default/pod",
		window: time.Nanosecond,
	},
}

func TestJitter(t *testing.T) {
	test.Map(t, jitterTestCases).
		Run(func(t test.Test, param jitterParams) {
			// Given
			delay := controller.NewDelay(0, param.window)

			// When
			result := delay.Jitter(param.key)

			// Then
			assert.Equal(t, param.expect, result)
		})
}

type jitterWindowParams struct {
	keys   []string
	window time.Duration
}

var jitterWindowTestCases = map[string]jitterWindowParams{
	"minute-window": {
		keys: []string{
			"default/pod-1", "default/pod-2", "other/pod-1", "other/pod-2",
		},
		window: time.Minute,
	},

	"millisecond-window": {
		keys:   []string{"default/pod-1", "default/pod-2"},
		window: time.Millisecond,
	},
}

func TestJitterWindow(t *testing.T) {
	test.Map(t, jitterWindowTestCases).
		Run(func(t test.Test, param jitterWindowParams) {
			// Given
			delay := controller.NewDelay(0, param.window)

			for _, key := range param.keys {
				// When
				offset := delay.Jitter(key)

				// Then
				assert.GreaterOrEqual(t, offset, time.Duration(0), key)
				assert.Less(t, offset, param.window, key)
				assert.Equal(t, offset, delay.Jitter(key), key)
			}
		})
}

type jitterSpreadParams struct {
	keys    int
	window  time.Duration
	buckets int64
	expect  int
}

// The offsets must not collapse into a few buckets, since this reproduces the
// re-sync spike the jitter is supposed to spread.
var jitterSpreadTestCases = map[string]jitterSpreadParams{
	"spread-over-minute": {
		keys:    100,
		window:  time.Minute,
		buckets: 10,
		expect:  10,
	},

	"spread-over-hour": {
		keys:    100,
		window:  time.Hour,
		buckets: 10,
		expect:  10,
	},
}

func TestJitterSpread(t *testing.T) {
	test.Map(t, jitterSpreadTestCases).
		Run(func(t test.Test, param jitterSpreadParams) {
			// Given
			delay := controller.NewDelay(0, param.window)

			// When
			buckets := map[int64]bool{}
			for index := range param.keys {
				offset := delay.Jitter(fmt.Sprintf("default/pod-%d", index))
				buckets[int64(offset)*param.buckets/
					int64(param.window)] = true
			}

			// Then
			assert.Len(t, buckets, param.expect)
		})
}

// TODO: integrate with tests.
func TestResource(t *testing.T) {
	client := fake.NewClientset()
	ctrl := controller.New[*corev1.Pod](Config("add-handler"),
		controller.NewRetriever(client.CoreV1().Pods(""), errTest),
		cache.Indexers{})
	assert.NotNil(t, ctrl)
}

type controllerNewParams struct {
	config   *controller.Config
	indexers cache.Indexers
}

var controllerNewTestCases = map[string]controllerNewParams{
	"success": {
		config:   Config("new"),
		indexers: cache.Indexers{},
	},
	"indexers": {
		config: Config("new-indexers"),
		indexers: cache.Indexers{
			"namespace": cache.MetaNamespaceIndexFunc,
		},
	},
	"multiple-workers": {
		config: &controller.Config{
			Name: "multi-worker", Workers: 3, Sync: time.Minute,
		},
		indexers: cache.Indexers{},
	},
}

func TestControllerNew(t *testing.T) {
	test.Map(t, controllerNewTestCases).
		Run(func(t test.Test, param controllerNewParams) {
			// Given
			mocks := mock.NewMocks(t)
			retriever := mock.Get(mocks, NewMockRetriever[*corev1.PodList])

			// When
			ctrl := controller.New[*corev1.Pod](
				param.config, retriever, param.indexers)

			// Then
			assert.NotNil(t, ctrl)
		})
}

type configLimiterParams struct {
	limiter controller.RateLimiter
	setup   mock.SetupFunc
}

var configLimiterTestCases = map[string]configLimiterParams{
	"default-limiter": {
		setup: CallRecorderLen("add-handler"),
	},

	"custom-limiter": {
		limiter: func() workqueue.TypedRateLimiter[string] {
			return workqueue.NewTypedItemFastSlowRateLimiter[string](
				time.Millisecond, time.Second, 1)
		},
		setup: CallRecorderLen("add-handler"),
	},
}

func TestConfigLimiter(t *testing.T) {
	test.Map(t, configLimiterTestCases).
		Run(func(t test.Test, param configLimiterParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			config := Config("add-handler")
			config.RateLimiter = param.limiter
			ctrl := controller.New[*corev1.Pod](config,
				mock.Get(mocks, NewMockRetriever[*corev1.PodList]),
				cache.Indexers{})

			// When
			err := ctrl.AddHandler("add-handler",
				mock.Get(mocks, NewMockHandler[*corev1.Pod]),
				mock.Get(mocks, NewMockRecorder))

			// Then
			assert.NoError(t, err)
		})
}

// podEventHandlers is the handler slice retained by the controller.
type podEventHandlers = []*controller.ResourceEventHandler[*corev1.Pod]

// handlerSpec captures the observable identity of a retained event handler.
// The handlers cannot be compared as a whole, since their queues wrap live
// channels and a condition variable.
type handlerSpec struct {
	handler controller.Handler[*corev1.Pod]
	queue   string
}

// handlerSpecs extracts the specs of the handlers retained by the controller.
func handlerSpecs(ctrl controller.Controller[*corev1.Pod]) []handlerSpec {
	handlers := test.Cast[podEventHandlers](
		reflect.NewAccessor(ctrl).Get("handler"))

	specs := make([]handlerSpec, 0, len(handlers))
	for _, handler := range handlers {
		accessor := reflect.NewAccessor(handler)
		specs = append(specs, handlerSpec{
			handler: test.Cast[controller.Handler[*corev1.Pod]](
				accessor.Get("handler")),
			queue: test.Cast[controller.Queue[string]](
				accessor.Get("queue")).Name(),
		})
	}

	return specs
}

// CallInformerAddHandler sets up the handler registration on the informer.
// The registered handler is created inside the controller and therefore only
// validated via the retained handler specs.
func CallInformerAddHandler(err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockSharedIndexInformer).EXPECT().
			AddEventHandlerWithResyncPeriod(gomock.Any(), time.Minute).
			Return(nil, err)
	}
}

// CallInformerGetIndexer sets up the indexer lookup on the informer.
func CallInformerGetIndexer() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockSharedIndexInformer).EXPECT().
			GetIndexer().Return(nil)
	}
}

// ErrAddHandler creates the error expected when registering the handler with
// the given name fails.
func ErrAddHandler(name string, err error) error {
	return controller.ErrController.New(
		"event handler [name=%s, handler=%s] %w", "add-handler", name, err)
}

type controllerAddHandlerParams struct {
	setup  mock.SetupFunc
	names  []string
	expect []string
	errors []error
}

var controllerAddHandlerTestCases = map[string]controllerAddHandlerParams{
	"success": {
		setup: mock.Chain(
			CallRecorderLen("add-handler"),
			CallInformerAddHandler(nil),
			CallInformerGetIndexer(),
		),
		names:  []string{"add-handler"},
		expect: []string{"add-handler"},
		errors: []error{nil},
	},

	"second-handler": {
		setup: mock.Chain(
			CallRecorderLen("add-handler"),
			CallInformerAddHandler(nil),
			CallInformerGetIndexer(),
			CallRecorderLen("other-handler"),
			CallInformerAddHandler(nil),
			CallInformerGetIndexer(),
		),
		names:  []string{"add-handler", "other-handler"},
		expect: []string{"add-handler", "other-handler"},
		errors: []error{nil, nil},
	},

	"error": {
		setup: mock.Chain(
			CallRecorderLen("add-handler"),
			CallInformerAddHandler(assert.AnError),
		),
		names:  []string{"add-handler"},
		expect: []string{},
		errors: []error{
			ErrAddHandler("add-handler", assert.AnError),
		},
	},

	"second-handler-error": {
		setup: mock.Chain(
			CallRecorderLen("add-handler"),
			CallInformerAddHandler(nil),
			CallInformerGetIndexer(),
			CallRecorderLen("other-handler"),
			CallInformerAddHandler(assert.AnError),
		),
		names:  []string{"add-handler", "other-handler"},
		expect: []string{"add-handler"},
		errors: []error{
			nil, ErrAddHandler("other-handler", assert.AnError),
		},
	},

	"both-error": {
		setup: mock.Chain(
			CallRecorderLen("add-handler"),
			CallInformerAddHandler(assert.AnError),
			CallRecorderLen("other-handler"),
			CallInformerAddHandler(assert.AnError),
		),
		names:  []string{"add-handler", "other-handler"},
		expect: []string{},
		errors: []error{
			ErrAddHandler("add-handler", assert.AnError),
			ErrAddHandler("other-handler", assert.AnError),
		},
	},
}

func TestControllerAddHandler(t *testing.T) {
	test.Map(t, controllerAddHandlerTestCases).
		Run(func(t test.Test, param controllerAddHandlerParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			ctrl := controller.New[*corev1.Pod](
				Config("add-handler"),
				mock.Get(mocks, NewMockRetriever[*corev1.PodList]),
				cache.Indexers{})
			reflect.NewAccessor(ctrl).Set("informer",
				mock.Get(mocks, NewMockSharedIndexInformer))
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			recorder := mock.Get(mocks, NewMockRecorder)

			// When
			errs := make([]error, 0, len(param.names))
			for _, name := range param.names {
				errs = append(errs, ctrl.AddHandler(name, handler, recorder))
			}

			// Then
			assert.Equal(t, param.errors, errs)
			expect := make([]handlerSpec, 0, len(param.expect))
			for _, queue := range param.expect {
				expect = append(expect, handlerSpec{
					handler: handler, queue: queue,
				})
			}
			assert.Equal(t, expect, handlerSpecs(ctrl))
		})
}

type controllerRunParams struct {
	config *controller.Config
	setup  mock.SetupFunc
	before func(ctrl controller.Controller[*corev1.Pod], mocks *mock.Mocks)
	expect error
}

var controllerRunTestCases = map[string]controllerRunParams{
	"success": {
		setup: mock.Chain(
			CallRetrieverWatchEndless[*corev1.PodList](),
			CallRetrieverList(NewPodList(p1, p2, p3, p4, p5, p6, p7), nil),
		),
	},
	"timeout": {
		setup: mock.Parallel(
			// TODO: find a better way to simulate timeout waiting for sync or
			// create a call function that blocks until context is done.
			func(mocks *mock.Mocks) any {
				return mock.Get(mocks, NewMockRetriever[*corev1.PodList]).
					EXPECT().List(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, _ metav1.ListOptions,
					) (runtime.Object, error) {
						<-ctx.Done()

						return nil, ctx.Err()
					})
			},
			func(mocks *mock.Mocks) any {
				return mock.Get(mocks, NewMockRetriever[*corev1.PodList]).EXPECT().
					Watch(gomock.Any(), gomock.Any()).AnyTimes().
					Return(nil, errors.New("watch not available"))
			},
		),
		expect: controller.ErrController.New("running [name=%s]: %s",
			"run", "timed out waiting for sync"),
	},
	"with-processor": {
		config: &controller.Config{
			Name: "run", Workers: 0, Sync: time.Minute,
		},
		setup: mock.Setup(
			mock.Chain(
				CallRetrieverWatchEndless[*corev1.PodList](),
				CallRecorderLen("run"),
				CallRetrieverList(NewPodList(p1, p2), nil),
			),
			CallRecorderAddEvent(),
			CallHandlerNotifyAny(),
		),
		before: func(ctrl controller.Controller[*corev1.Pod], mocks *mock.Mocks) {
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			recorder := mock.Get(mocks, NewMockRecorder)
			_ = ctrl.AddHandler("run", handler, recorder)
		},
	},

	"with-worker": {
		config: &controller.Config{
			Name: "run", Workers: 1, Sync: time.Minute,
		},
		setup: mock.Setup(
			mock.Chain(
				CallRetrieverWatchEndless[*corev1.PodList](),
				CallRecorderLen("run"),
				CallRetrieverList(NewPodList(p1, p2), nil),
			),
			CallRecorderAddEvent(),
			CallRecorderEventAny(),
			CallHandlerHandleAny(),
			CallHandlerNotifyAny(),
		),
		before: func(ctrl controller.Controller[*corev1.Pod], mocks *mock.Mocks) {
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			recorder := mock.Get(mocks, NewMockRecorder)
			_ = ctrl.AddHandler("run", handler, recorder)
		},
	},
}

func TestControllerRun(t *testing.T) {
	test.Map(t, controllerRunTestCases).
		// Filter(test.Pattern[controllerRunParams]("success")).
		Run(func(t test.Test, param controllerRunParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			config := param.config
			if config == nil {
				config = Config("run")
			}
			retriever := mock.Get(mocks, NewMockRetriever[*corev1.PodList])
			ctrl := controller.New[*corev1.Pod](
				config, retriever, cache.Indexers{})
			if param.before != nil {
				param.before(ctrl, mocks)
			}

			sigerr := make(chan error, 1)
			var ctx context.Context
			var cancel context.CancelFunc
			if param.expect != nil {
				ctx, cancel = context.WithTimeout(
					context.Background(), 50*time.Millisecond)
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()

			// When
			ctrl.Init(ctx, sigerr)
			go ctrl.Run(ctx)

			// Then
			timeout := 100 * time.Millisecond
			if param.expect == nil {
				time.Sleep(timeout)
				cancel()
				// Drain any error from context cancellation
				select {
				case <-sigerr:
				case <-time.After(timeout):
				}
			} else {
				select {
				case err := <-sigerr:
					assert.Equal(t, param.expect, err)
				case <-time.After(timeout):
					t.Fatal("timeout waiting for run result")
				}
			}
		})
}

type controllerGetParams struct {
	key     string
	setup   mock.SetupFunc
	indexer cache.Indexer
	expect  *corev1.Pod
	error   error
}

var controllerGetTestCases = map[string]controllerGetParams{
	"absent": {
		key:     "default/absent",
		indexer: NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
	},
	"match": {
		key:     "default/pod-owner",
		indexer: NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
		expect:  p2,
	},
	"mismatch": {
		key:     "default/dummy",
		indexer: NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
		error: controller.ErrController.New("type [name=%s, key=%s]: %T",
			"get", "default/dummy", d1),
	},
	"error": {
		key:   "default/error",
		setup: CallGetByKey("default/error", nil, false, assert.AnError),
		error: controller.ErrController.New("get by key [name=%s, key=%s]: %w",
			"get", "default/error", assert.AnError),
	},
}

func TestControllerGet(t *testing.T) {
	test.Map(t, controllerGetTestCases).
		Run(func(t test.Test, param controllerGetParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			retriever := mock.Get(mocks, NewMockRetriever[*corev1.PodList])
			ctrl := controller.New[*corev1.Pod](
				Config("get"), retriever, cache.Indexers{})
			reflect.NewAccessor(reflect.NewAccessor(ctrl).Get("informer")).
				Set("indexer", GetIndexer(mocks, param.indexer))

			// When
			result, err := ctrl.Get(param.key)

			// Then
			assert.Equal(t, param.error, err)
			assert.Equal(t, param.expect, result)
		})
}

type controllerListParams struct {
	setup     mock.SetupFunc
	namespace string
	name      string
	uid       types.UID
	indexer   cache.Indexer
	expect    []*corev1.Pod
}

var controllerListTestCases = map[string]controllerListParams{
	"empty": {
		indexer: NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
		expect:  []*corev1.Pod{p1, p2, p3, p4, p5, p6, p7},
	},
	"missing-name": {
		name:    "missing",
		indexer: NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
		expect:  []*corev1.Pod{},
	},
	"missing-id": {
		uid:     "missing",
		indexer: NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
		expect:  []*corev1.Pod{},
	},
	"match-all": {
		namespace: "default",
		name:      "owner",
		uid:       "owner-id",
		indexer:   NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
		expect:    []*corev1.Pod{p2},
	},
	"space-name": {
		namespace: "default",
		name:      "owner",
		indexer:   NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
		expect:    []*corev1.Pod{p2, p4},
	},
	"space-id": {
		namespace: "default",
		uid:       "owner-id",
		indexer:   NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
		expect:    []*corev1.Pod{p2, p3},
	},
	"space-only": {
		namespace: "default",
		indexer:   NewIndexer(d1, p1, p2, p3, p4, p5, p6, p7),
		expect:    []*corev1.Pod{p1, p2, p3, p4, p5},
	},

	// Indexed lookups using the registered owner indexers.
	"index-by-uid": {
		namespace: "default",
		uid:       "owner-id",
		indexer:   NewOwnerIndexer(p1, p2, p3, p4, p5, p6, p7),
		expect:    []*corev1.Pod{p2, p3},
	},

	"index-by-name": {
		namespace: "default",
		name:      "owner",
		indexer:   NewOwnerIndexer(p1, p2, p3, p4, p5, p6, p7),
		expect:    []*corev1.Pod{p2, p4},
	},

	"index-by-uid-and-name": {
		namespace: "default",
		name:      "owner",
		uid:       "owner-id",
		indexer:   NewOwnerIndexer(p1, p2, p3, p4, p5, p6, p7),
		expect:    []*corev1.Pod{p2},
	},

	"index-without-owner": {
		namespace: "default",
		indexer:   NewOwnerIndexer(p1, p2, p3, p4, p5, p6, p7),
		expect:    []*corev1.Pod{p1, p2, p3, p4, p5},
	},
}

func TestControllerList(t *testing.T) {
	test.Map(t, controllerListTestCases).
		Run(func(t test.Test, param controllerListParams) {
			// Given
			log.SetLevel(log.TraceLevel)
			mocks := mock.NewMocks(t).Expect(param.setup)
			retriever := mock.Get(mocks, NewMockRetriever[*corev1.PodList])
			ctrl := controller.New[*corev1.Pod](
				Config("list"), retriever, cache.Indexers{})
			reflect.NewAccessor(reflect.NewAccessor(ctrl).Get("informer")).
				Set("indexer", GetIndexer(mocks, param.indexer))

			// When
			result := ctrl.List(param.namespace, param.name, param.uid)

			// Then
			assert.ElementsMatch(t, param.expect, result)
		}).
		Cleanup(func() {
			log.SetLevel(log.InfoLevel)
		})
}

type controllerListByIndexParams struct {
	index   string
	value   string
	indexer cache.Indexer
	expect  []*corev1.Pod
}

var controllerListByIndexTestCases = map[string]controllerListByIndexParams{
	"by-owner-uid": {
		index:   controller.IndexOwnerUID,
		value:   "owner-id",
		indexer: NewOwnerIndexer(p1, p2, p3, p4, p5, p6, p7),
		expect:  []*corev1.Pod{p2, p3, p7},
	},

	"by-owner-name": {
		index:   controller.IndexOwnerName,
		value:   "owner",
		indexer: NewOwnerIndexer(p1, p2, p3, p4, p5, p6, p7),
		expect:  []*corev1.Pod{p2, p4, p7},
	},

	"unknown-value": {
		index:   controller.IndexOwnerUID,
		value:   "missing",
		indexer: NewOwnerIndexer(p1, p2, p3, p4, p5, p6, p7),
		expect:  []*corev1.Pod{},
	},

	"unknown-index": {
		index:   "missing",
		value:   "owner-id",
		indexer: NewOwnerIndexer(p1, p2, p3, p4, p5, p6, p7),
		expect:  []*corev1.Pod{},
	},
}

func TestControllerListByIndex(t *testing.T) {
	test.Map(t, controllerListByIndexTestCases).
		Run(func(t test.Test, param controllerListByIndexParams) {
			// Given
			mocks := mock.NewMocks(t)
			retriever := mock.Get(mocks, NewMockRetriever[*corev1.PodList])
			ctrl := controller.New[*corev1.Pod](
				Config("list-index"), retriever, cache.Indexers{})
			reflect.NewAccessor(reflect.NewAccessor(ctrl).Get("informer")).
				Set("indexer", GetIndexer(mocks, param.indexer))

			// When
			result := ctrl.ListByIndex(param.index, param.value)

			// Then
			assert.ElementsMatch(t, param.expect, result)
		})
}
