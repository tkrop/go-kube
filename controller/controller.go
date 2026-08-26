package controller

import (
	"context"
	"hash/fnv"
	"math"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	log "github.com/sirupsen/logrus"
	"github.com/tkrop/go-kube/errors"
)

// ErrController is an operator error.
var ErrController = errors.New("controller")

// RateLimiter creates the back-off strategy used to delay requeued events.
type RateLimiter func() workqueue.TypedRateLimiter[string]

// Config is the controller configuration. The values are used as provided -
// defaults are supposed to be supplied by the config setup of the consumer.
type Config struct {
	// Name of the main controller resource.
	Name string
	// Workers is the number of concurrent workers the controller will run
	// processing events. Values below one start no workers and thereby only
	// cache the resources without processing events.
	Workers int
	// Sync is the interval in which the controller will process a re-capture
	// of the selected resources from the API server. Zero disables the
	// periodic re-sync.
	Sync time.Duration
	// Retries is the number of times the controller retries processing of an
	// event after a failure. Zero drops the event on the first failure, while
	// a negative value retries indefinitely. Once the retries are exhausted
	// the event is dropped and recovery depends on the next resource event or
	// re-sync.
	Retries int
	// Delay defines the delays applied when enqueuing resource events to
	// coalesce repeated events and to spread the periodic re-sync.
	Delay Delay
	// RateLimiter creates the back-off strategy used to delay requeued events.
	// It is called once per handler to avoid sharing back-off state between
	// queues. If unset, `workqueue.DefaultTypedControllerRateLimiter` is used,
	// that starts at a 5ms delay doubling it up to 1000s.
	RateLimiter RateLimiter
}

// Delay defines the delays applied when enqueuing resource events.
type Delay struct {
	// Debounce delays the enqueuing of resource events to coalesce repeated
	// events of the same resource into a single reconcile. The delaying queue
	// keeps the earliest deadline of a resource, i.e. the events are coalesced
	// on the leading edge. Zero enqueues the events immediately.
	Debounce time.Duration
	// Resync spreads the enqueuing of the resource events created by the
	// periodic re-sync over the given window using a stable per resource
	// jitter. This avoids the reconcile spike created by replaying the whole
	// cache at once. Zero falls back to the debounce delay.
	Resync time.Duration
}

// NewDelay creates the delays applied when enqueuing resource events using
// the given debounce delay and re-sync window.
func NewDelay(debounce, resync time.Duration) Delay {
	return Delay{
		Debounce: debounce,
		Resync:   resync,
	}
}

// Jitter returns a stable offset within the re-sync window derived from the
// given resource key. Deriving the offset from the key spreads the resources
// evenly over the window while keeping the offset of a resource reproducible.
func (d Delay) Jitter(key string) time.Duration {
	if d.Resync <= 0 {
		return 0
	}

	hash := fnv.New64a()
	// Writing to a hash never fails.
	_, _ = hash.Write([]byte(key))

	// Masking the sign bit keeps the offset positive.
	return time.Duration(hash.Sum64()&math.MaxInt64) % d.Resync
}

// delay returns the delay applied when enqueuing the event of the resource
// with the given key.
func (d Delay) delay(key string, resync bool) time.Duration {
	if resync && d.Resync > 0 {
		return d.Jitter(key)
	}

	return d.Debounce
}

// Controller is the interface for managing the controller and accessing
// controller resources.
type Controller[T runtime.Object] interface {
	// Runnable enables the controller to be runnable.
	Runnable
	// Get retrieves an resource object given by its key.
	Get(key string) (T, error)
	// List retrieves all objects owned by the owner with the given namespace,
	// object name, and given uid. If name or uid are empty, they are ignored
	// during the ownership check. If the namespace is empty, all namespaces are
	// considered. If the namespace, name, and uid are empty, all objects are
	// returned.
	List(namespace, name string, uid types.UID) []T
	// ListByIndex retrieves all objects stored under the given value in the
	// index with the given name. It retrieves no objects, if the index is not
	// registered via the indexers provided to `New`.
	ListByIndex(index, value string) []T
	// AddHandler will add a new handler with the given name to the controller
	// that only receives the resource events passing all given filters. The
	// name is used to distinguish the handler queues in the metrics. Without
	// filters all events are enqueued.
	AddHandler(
		name string, handler Handler[T], recorder Recorder, filter ...Filter,
	) error
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
// and indexers. To narrow the observed resources by label or field selectors,
// apply them to the list options in the given retriever. To look up resources
// by owner without scanning the whole cache, register the `OwnerIndexers`.
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

// AddHandler will add a new handler with the given name to the controller
// that only receives the resource events passing all given filters. The name
// is used to distinguish the handler queues in the metrics. Without filters
// all events are enqueued.
func (c *controller[T]) AddHandler(
	name string, handler Handler[T], recorder Recorder, filter ...Filter,
) error {
	// TODO: check whether we can simplify the rate limiter creation?
	limiter := c.config.RateLimiter
	if limiter == nil {
		limiter = workqueue.DefaultTypedControllerRateLimiter[string]
	}

	queue := NewRateLimitedQueue(name, limiter(),
		c.config.Retries, recorder)

	if err := c.addHandler(name, NewResourceEventHandler[T](
		handler, queue, c.config.Delay, filter...)); err != nil {
		return err
	}

	c.processor = append(c.processor, NewProcessor[T](
		handler, c.informer, c.config.Workers, queue, recorder))

	return nil
}

// addHandler adds the given resource event handler to the informer.
func (c *controller[T]) addHandler(
	name string, handler *ResourceEventHandler[T],
) error {
	if _, err := c.informer.AddEventHandlerWithResyncPeriod(
		handler, c.config.Sync); err != nil {
		return ErrController.New("event handler [name=%s, handler=%s] %w",
			c.config.Name, name, err)
	}

	c.handler = append(c.handler, handler)

	return nil
}

// Init initializes the controller loop by starting the informer and waiting
// until the cache is synced. This allows to coordinate multiple controllers by
// initializing all controllers before running their processors.
func (c *controller[T]) Init(ctx context.Context, errch chan error) {
	go c.informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		errch <- ErrController.New("running [name=%s]: %s",
			c.config.Name, "timed out waiting for sync")
	}
}

// Run starts the controller loop by creating the workers that are running the
// actual event processing.
func (c *controller[T]) Run(ctx context.Context) {
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
	values := c.lookup(name, uid)
	results := make([]T, 0, len(values))
	for _, value := range values {
		if result, ok := value.(T); ok &&
			c.owner(namespace, name, uid, result) {
			results = append(results, result)
		}
	}

	if log.IsLevelEnabled(log.TraceLevel) {
		log.WithFields(log.Fields{
			"namespace": namespace,
			"name":      name,
			"uid":       uid,
			"results":   len(results),
		}).Tracef("listing %s", c.config.Name)
	}

	return results
}

// lookup collects the candidate objects for the owner with the given object
// name and uid using the most selective owner index registered via the
// indexers provided to `New`. It falls back to scanning the whole cache, if
// no suitable index is registered.
func (c *controller[T]) lookup(name string, uid types.UID) []any {
	indexer := c.informer.GetIndexer()

	if uid != "" {
		if values, err := indexer.ByIndex(
			IndexOwnerUID, string(uid)); err == nil {
			return values
		}
	}

	if name != "" {
		if values, err := indexer.ByIndex(
			IndexOwnerName, name); err == nil {
			return values
		}
	}

	return indexer.List()
}

// ListByIndex retrieves all objects stored under the given value in the index
// with the given name. It retrieves no objects, if the index is not registered
// via the indexers provided to `New`.
func (c *controller[T]) ListByIndex(index, value string) []T {
	values, err := c.informer.GetIndexer().ByIndex(index, value)
	if err != nil {
		return []T{}
	}

	results := make([]T, 0, len(values))
	for _, value := range values {
		if result, ok := value.(T); ok {
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
