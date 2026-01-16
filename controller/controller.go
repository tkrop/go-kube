package controller

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/tkrop/go-kube/errors"
)

// ErrController is an operator error.
var ErrController = errors.NewError("controller")

// Config is the controller configuration.
type Config struct {
	// Name of the main controller resource.
	Name string
	// Workers is the number of concurrent workers the controller will run
	// processing events.
	Workers int
	// Sync is the interval in which the controller will process a re-capture
	// of the selected resources from the API server.
	Sync time.Duration
	// Retries is the number of times the controller will try to process an
	// resource event before returning a real error.
	Retries int
}

// Controller is the interface for managing the controller and accessing
// controller resources.
type Controller[T runtime.Object] interface {
	// Get retrieves an resource object given by its key.
	Get(key string) (T, error)
	// List retrieves all objects owned by the owner with the given namespace,
	// object name, and given uid. If name or uid are empty, they are ignored
	// during the ownership check. If the namespace is empty, all namespaces are
	// considered. If the namespace, name, and uid are empty, all objects are
	// returned.
	List(namespace, name string, uid types.UID) []T
	// AddHandler will add a new handler to the controller.
	AddHandler(handler Handler[T], recorder Recorder) error
	// Run starts the controller loop by starting the informer, waiting until
	// the cache is syncing, before creating the worker pools that are running
	// the processors.
	Run(ctx context.Context, errch chan error)
}

// controller is the implementation of the cache interface.
type controller[T runtime.Object] struct {
	// The controller configuration.
	config *Config
	// The shared informer created by the controller.
	informer cache.SharedIndexInformer
	// The wrapped retriever used by shared index informer.
	handler []*ResourceEventHandler[T]
	// The list of processors created by the controller.
	processor []*Processor[T]
}

// New creates a new controller for given retriever using given configuration
// and indexers.
func New[T runtime.Object, L runtime.Object](
	config *Config, retriever Retriever[L], indexers cache.Indexers,
) Controller[T] {
	var temp T
	informer := cache.NewSharedIndexInformer(&cache.ListWatch{
		ListWithContextFunc: func(
			ctx context.Context, options metav1.ListOptions,
		) (runtime.Object, error) {
			return retriever.List(ctx, options)
		},
		WatchFuncWithContext: retriever.Watch,
	}, temp, config.Sync, indexers)

	return &controller[T]{
		config:    config,
		informer:  informer,
		processor: []*Processor[T]{},
	}
}

// AddHandler will add a new handler to the controller.
func (c *controller[T]) AddHandler(
	handler Handler[T], recorder Recorder,
) error {
	queue := NewDefaultQueue(c.config.Name, c.config.Retries, recorder)

	if err := c.addHandler(
		NewResourceEventHandler[T](handler, queue)); err != nil {
		return err
	}

	c.processor = append(c.processor, NewProcessor[T](
		handler, c.informer, c.config.Workers, queue, recorder))

	return nil
}

// addHandler adds the given resource event handler to the informer.
func (c *controller[T]) addHandler(handler *ResourceEventHandler[T]) error {
	c.handler = append(c.handler, handler)

	if _, err := c.informer.AddEventHandlerWithResyncPeriod(
		handler, c.config.Sync); err != nil {
		return ErrController.New("event handler [name=%s] %w",
			c.config.Name, err)
	}

	return nil
}

// Run starts the controller loop by starting the informer, waiting until the
// cache is syncing, before creating the worker pools that are running the
// processors.
func (c *controller[T]) Run(ctx context.Context, errch chan error) {
	go c.informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		errch <- ErrController.New("running [name=%s]: %s",
			c.config.Name, "timed out waiting for sync")
	}

	for _, processor := range c.processor {
		processor.Run(ctx)
	}
}

// Get retrieves an resource object given by its key.
func (c *controller[T]) Get(key string) (T, error) {
	var zero T
	value, exists, err := c.informer.GetIndexer().GetByKey(key)
	if err != nil {
		return zero, ErrController.New("get by key [name=%s, key=%s]: %w",
			c.config.Name, key, err)
	} else if result, ok := value.(T); !ok && exists {
		return zero, ErrController.New("type [name=%s, key=%s]: %T",
			c.config.Name, key, value)
	} else {
		return result, nil
	}
}

// List retrieves all objects owned by the owner with the given namespace,
// object name, and given uid. If name or uid are empty, they are ignored
// during the ownership check. If the namespace is empty, all namespaces are
// considered. If the namespace, name, and uid are empty, all objects are
// returned.
func (c *controller[T]) List(namespace, name string, uid types.UID) []T {
	results := []T{}
	for _, value := range c.informer.GetIndexer().List() {
		if result, ok := value.(T); ok &&
			c.owner(namespace, name, uid, result) {
			results = append(results, result)
		}
	}

	return results
}

// owner checks whether the given object is owned by the given owner.
func (*controller[T]) owner(
	namespace, name string, uid types.UID, result any,
) bool {
	if access, ok := result.(metav1.ObjectMetaAccessor); ok {
		meta := access.GetObjectMeta()
		if namespace != "" &&
			meta.GetNamespace() != namespace {
			return false
		}
		if name == "" && uid == "" {
			return true
		}

		for _, oref := range meta.GetOwnerReferences() {
			if (uid == "" || uid == oref.UID) &&
				(name == "" || name == oref.Name) {
				return true
			}
		}
	}

	return false
}
