package controller

import (
	"strconv"

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

// Filter decides whether an observed resource event is enqueued for
// processing. The previous resource is only provided on update events, and
// the resource is nil, if it cannot be accessed, e.g. for deletions observed
// via a tombstone.
type Filter func(op Op, prev, obj runtime.Object) bool

// GenerationChanged creates a filter that only passes update events changing
// the resource generation. Since the generation is only advanced on spec
// changes of resources with a status sub-resource, this drops the events
// created by the controller writing the status of a resource it is watching.
// Add and delete events always pass.
func GenerationChanged() Filter {
	return changed(func(meta metav1.Object) string {
		return strconv.FormatInt(meta.GetGeneration(), 10)
	})
}

// ResourceVersionChanged creates a filter that only passes update events
// changing the resource version. This drops the no-op update events created
// by the periodic re-sync. Add and delete events always pass.
func ResourceVersionChanged() Filter {
	return changed(metav1.Object.GetResourceVersion)
}

// changed creates a filter that only passes update events changing the value
// provided by the given accessor.
func changed(value func(metav1.Object) string) Filter {
	return func(op Op, prev, obj runtime.Object) bool {
		if op != OpUpdate {
			return true
		}

		before, after := meta(prev), meta(obj)
		if before == nil || after == nil {
			return true
		}

		return value(before) != value(after)
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

// meta retrieves the object meta of the given resource. It returns nil, if
// the resource does not provide an object meta.
func meta(obj runtime.Object) metav1.Object {
	if access, ok := obj.(metav1.ObjectMetaAccessor); ok {
		return access.GetObjectMeta()
	}

	return nil
}
