package controller_test

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"go.uber.org/mock/gomock"

	"github.com/tkrop/go-testing/mock"
	"github.com/tkrop/go-testing/test"

	"github.com/tkrop/go-kube/controller"
)

// Config creates a basic controller Config for testing.
func Config(name string) *controller.Config {
	return &controller.Config{
		Name: name, Workers: 1, Sync: time.Minute,
	}
}

// Object is a simple implementation of a Kubernetes object.
type Object struct {
	//revive:disable-next-line:struct-tag // kubernetes json tag customization.
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// DeepCopyObject creates a deep copy of the object.
func (o *Object) DeepCopyObject() runtime.Object {
	out := *o

	return &out
}

// GetObjectKind retrieves the object kind.
func (o *Object) GetObjectKind() schema.ObjectKind {
	return &o.TypeMeta
}

// List is a simple implementation of a Kubernetes list object.
type List struct {
	//revive:disable-next-line:struct-tag // kubernetes json tag customization.
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []runtime.Object `json:"items"`
}

// NewList creates a new list object with given items.
func NewList(items ...runtime.Object) *List {
	return &List{
		Items: items,
	}
}

// DeepCopyObject creates a deep copy of the list object.
func (l *List) DeepCopyObject() runtime.Object {
	copyItems := make([]runtime.Object, len(l.Items))
	for i, item := range l.Items {
		copyItems[i] = item.DeepCopyObject()
	}

	return &List{
		TypeMeta: l.TypeMeta,
		ListMeta: l.ListMeta,
		Items:    copyItems,
	}
}

// GetObjectKind retrieves the object kind.
func (l *List) GetObjectKind() schema.ObjectKind {
	return &l.TypeMeta
}

// NewPodList creates a new PodList with given Pod items.
func NewPodList(items ...any) *corev1.PodList {
	pods := make([]corev1.Pod, 0, len(items))
	for _, item := range items {
		if pod, ok := item.(*corev1.Pod); ok && pod != nil {
			pods = append(pods, *pod)
		}
	}

	return &corev1.PodList{
		Items: pods,
	}
}

// *** Indexer mocks setup functions. ***

// GetIndexer retrieves the indexer from mocks if no indexer is provided.
func GetIndexer(mocks *mock.Mocks, indexer cache.Indexer) cache.Indexer {
	if indexer == nil {
		return mock.Get(mocks, NewMockIndexer)
	}

	return indexer
}

// NewIndexer creates an indexer populated with elements.
func NewIndexer(items ...any) cache.Indexer {
	indexer := cache.NewIndexer(
		cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for index, item := range items {
		if err := indexer.Add(item); err != nil {
			panic(fmt.Errorf("indexer add [%d=%v]: %w",
				index, item, err))
		}
	}

	return indexer
}

// CallGetByKey sets up a mock call to GetByKey on the indexer.
func CallGetByKey(
	key string, result any, exists bool, err error,
) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockIndexer).EXPECT().
			GetByKey(key).Return(result, exists, err)
	}
}

// *** Retriever mocks setup functions. ***

// CallRetrieverList sets up the expectation for the retriever list call.
func CallRetrieverList[T runtime.Object](list T, err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRetriever[T]).EXPECT().
			List(gomock.Any(), gomock.Any()).Return(list, err)
	}
}

// CallRetrieverWatchEndless sets up the expectation for the retriever watch
// call to repeat endlessly.
//
// TODO: while this works, it would be better to have a way to block after
// the first call to the watcher, so that we can control when to continue and
// stop it in the tests to avoid endless repetition.
func CallRetrieverWatchEndless[T runtime.Object]() mock.SetupFunc {
	return mock.Setup(
		func(mocks *mock.Mocks) any {
			return test.Cast[*gomock.Call](
				CallRetrieverWatch[T](nil)(mocks)).AnyTimes()
		},
		func(mocks *mock.Mocks) any {
			return test.Cast[*gomock.Call](
				CallWatcherResult()(mocks)).AnyTimes()
		},
		func(mocks *mock.Mocks) any {
			return test.Cast[*gomock.Call](
				CallWatcherStop()(mocks)).AnyTimes()
		},
	)
}

// CallRetrieverWatchStop sets up the expectation for the retriever watch call.
func CallRetrieverWatchStop[T runtime.Object](
	err error, events ...watch.Event,
) mock.SetupFunc {
	return mock.Chain(
		CallRetrieverWatch[T](err),
		CallWatcherResult(events...),
		CallWatcherStop(),
	)
}

// CallRetrieverWatch sets up the expectation for the retriever watch call.
func CallRetrieverWatch[T runtime.Object](err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRetriever[T]).EXPECT().
			Watch(gomock.Any(), gomock.Any()).
			Return(mock.Get(mocks, NewMockWatcher), err)
	}
}

// *** Watcher mocks setup functions. ***

// CallWatcherStop sets up the expectation for the watcher stop call.
func CallWatcherStop() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockWatcher).EXPECT().Stop()
	}
}

// CallWatcherResult sets up the expectation for the watcher result chan call.
func CallWatcherResult(events ...watch.Event) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		ch := make(chan watch.Event, len(events))
		go func() {
			for _, event := range events {
				ch <- event
			}
			close(ch)
		}()

		return mock.Get(mocks, NewMockWatcher).EXPECT().
			ResultChan().Return(ch)
	}
}
