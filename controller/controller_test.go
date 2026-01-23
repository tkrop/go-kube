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

type controllerAddHandlerParams struct {
	setup  mock.SetupFunc
	before func(ctrl controller.Controller[*corev1.Pod])
	expect func(ctrl controller.Controller[*corev1.Pod]) error
}

var controllerAddHandlerTestCases = map[string]controllerAddHandlerParams{
	"success": {
		setup: CallRecorderLen("add-handler"),
	},
	"error": {
		setup: mock.Chain(
			CallRetrieverWatchEndless[*corev1.PodList](),
			CallRetrieverList(NewPodList(p1, p2), nil),
			CallRecorderLen("add-handler"),
		),
		before: func(ctrl controller.Controller[*corev1.Pod]) {
			ctx, cancel := context.WithCancel(context.Background())
			sigerr := make(chan error, 1)
			ctrl.Init(ctx, sigerr)
			go ctrl.Run(ctx)
			time.Sleep(100 * time.Millisecond)
			cancel()
			// Wait with timeout for controller to shut down
			select {
			case <-sigerr:
				// Controller shut down cleanly
			case <-time.After(200 * time.Millisecond):
				// Continue anyway - controller might still be shutting down
			}
			time.Sleep(50 * time.Millisecond)
		},
		expect: func(ctrl controller.Controller[*corev1.Pod]) error {
			return controller.ErrController.New("event handler [name=%s] %w",
				"add-handler", fmt.Errorf("handler %v was not added to "+
					"shared informer because it has stopped already",
					test.Cast[[]*controller.ResourceEventHandler[*corev1.Pod]](
						reflect.NewAccessor(ctrl).Get("handler"))[0]))
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
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			recorder := mock.Get(mocks, NewMockRecorder)
			if param.before != nil {
				param.before(ctrl)
			}

			// When
			err := ctrl.AddHandler(handler, recorder)

			// Then
			if param.expect != nil {
				assert.Equal(t, param.expect(ctrl), err)
			} else {
				assert.Nil(t, err)
			}
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
		setup: mock.Chain(
			CallRetrieverWatchEndless[*corev1.PodList](),
			CallRecorderLen("run"),
			CallRetrieverList(NewPodList(p1, p2), nil),
			CallRecorderAddEvent(),
			CallHandlerNotifyAny(),
		),
		before: func(ctrl controller.Controller[*corev1.Pod], mocks *mock.Mocks) {
			handler := mock.Get(mocks, NewMockHandler[*corev1.Pod])
			recorder := mock.Get(mocks, NewMockRecorder)
			_ = ctrl.AddHandler(handler, recorder)
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
