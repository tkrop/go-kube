package controller

import (
	"strconv"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Op is the resource event operation observed by a filter.
type Op int

const (
	// OpAdd marks an event of a resource added to the cache.
	OpAdd Op = iota
	// OpUpdate marks an event of a resource updated in the cache.
	OpUpdate
	// OpDelete marks an event of a resource deleted from the cache.
	OpDelete
)

// String returns the name of the resource event operation.
func (op Op) String() string {
	switch op {
	case OpAdd:
		return "add"
	case OpUpdate:
		return "update"
	case OpDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// And creates a filter that passes an event, if all given filters pass it.
func And(filters ...Filter) Filter {
	return func(op Op, prev, obj runtime.Object) bool {
		for _, filter := range filters {
			if !filter(op, prev, obj) {
				return false
			}
		}

		return true
	}
}

// Or creates a filter that passes an event, if any given filter passes it.
func Or(filters ...Filter) Filter {
	return func(op Op, prev, obj runtime.Object) bool {
		for _, filter := range filters {
			if filter(op, prev, obj) {
				return true
			}
		}

		return false
	}
}

// Not creates a filter that passes an event, if the given filter drops it.
func Not(filter Filter) Filter {
	return func(op Op, prev, obj runtime.Object) bool {
		return !filter(op, prev, obj)
	}
}

// Filter decides whether an observed resource event is enqueued for
// processing. The previous resource is only provided on update events, and
// the resource is nil, if it cannot be accessed, e.g. for deletions observed
// via a tombstone.
type Filter func(op Op, prev, obj runtime.Object) bool

// GenerationChanged only passes update events changing the resource
// generation. Since the generation is only advanced on spec changes of
// resources with a status sub-resource, this drops the events created by the
// controller writing the status of a resource it is watching. Add and delete
// events always pass.
func GenerationChanged(op Op, prev, obj runtime.Object) bool {
	if op != OpUpdate {
		return true
	}

	before, after := meta(prev), meta(obj)
	if before == nil || after == nil {
		return true
	}

	return strconv.FormatInt(before.GetGeneration(), 10) !=
		strconv.FormatInt(after.GetGeneration(), 10)
}

// ResourceVersionChanged only passes update events changing the resource
// version. This drops the no-op update events created by the periodic re-sync.
// Add and delete events always pass.
func ResourceVersionChanged(op Op, prev, obj runtime.Object) bool {
	if op != OpUpdate {
		return true
	}

	before, after := meta(prev), meta(obj)
	if before == nil || after == nil {
		return true
	}

	return before.GetResourceVersion() != after.GetResourceVersion()
}

// meta retrieves the object meta of the given resource. It returns nil, if
// the resource does not provide an object meta.
func meta(obj runtime.Object) metav1.Object {
	if access, ok := obj.(metav1.ObjectMetaAccessor); ok {
		return access.GetObjectMeta()
	}

	return nil
}

// selfWriteEntry stores the recorded resource version and timestamp for a
// marked write.
type selfWriteEntry struct {
	version   string
	timestamp time.Time
}

// SelfWriteTracker suppresses redundant watch events resulting from the
// controller's own write operations. After the controller writes a status
// update to a resource, the resulting watch event is delivered back to the
// same handler. This tracker suppresses those self-inflicted echo events by
// recording the resource version immediately after a successful write, then
// dropping the matching incoming update event.
type SelfWriteTracker struct {
	entries map[string]selfWriteEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

// NewSelfWriteTracker creates a new tracker with the given TTL. A non-positive
// TTL falls back to the default of 5 minutes. The TTL ensures that stale entries
// from writes that were never echoed back (e.g., due to object deletion or
// superseding writes) are automatically cleaned up.
func NewSelfWriteTracker(ttl time.Duration) *SelfWriteTracker {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	return &SelfWriteTracker{
		entries: make(map[string]selfWriteEntry),
		ttl:     ttl,
	}
}

// Mark records the resource version of an object immediately after a
// successful write operation. The namespace and name form a unique key for
// tracking. This method is safe for concurrent use from multiple goroutines.
func (t *SelfWriteTracker) Mark(obj metav1.Object) {
	if obj == nil {
		return
	}

	key := obj.GetNamespace() + "/" + obj.GetName()
	version := obj.GetResourceVersion()

	t.mu.Lock()
	defer t.mu.Unlock()

	t.entries[key] = selfWriteEntry{
		version:   version,
		timestamp: time.Now(),
	}
}

// Filter implements the Filter type for SelfWriteTracker. It suppresses update
// events matching marked writes. On update events, if the object's namespace/name
// matches a marked write and the resource version matches exactly, the event is
// dropped and the entry is consumed (one-shot). Add and delete events always pass
// through, with delete events also clearing any tracked entry for that key to
// prevent memory leaks. Stale entries older than the TTL are treated as expired
// and do not suppress incoming events.
func (t *SelfWriteTracker) Filter(op Op, _, obj runtime.Object) bool {
	if op == OpDelete {
		// Always pass delete events, but clean up the tracked entry.
		objMeta := meta(obj)
		if objMeta != nil {
			key := objMeta.GetNamespace() + "/" + objMeta.GetName()

			t.mu.Lock()
			delete(t.entries, key)
			t.mu.Unlock()
		}

		return true
	}

	if op != OpUpdate {
		return true
	}

	// For update events, check if this matches a self-written resource
	// version and suppress it if so.
	objMeta := meta(obj)
	if objMeta == nil {
		return true
	}

	key := objMeta.GetNamespace() + "/" + objMeta.GetName()
	version := objMeta.GetResourceVersion()

	t.mu.Lock()
	defer t.mu.Unlock()

	entry, found := t.entries[key]
	if !found {
		return true
	}

	// Check if the entry has expired.
	if time.Since(entry.timestamp) > t.ttl {
		delete(t.entries, key)

		return true
	}

	// If the resource version matches, this is our echo. Consume it by
	// removing the entry and dropping the event.
	if entry.version == version {
		delete(t.entries, key)

		return false
	}

	// Version mismatch means this is a different event (e.g., a concurrent
	// write to the same resource). Pass it through and keep the entry for
	// later comparison.
	return true
}
