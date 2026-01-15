package controller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	log "github.com/sirupsen/logrus"

	"github.com/tkrop/go-kube/errors"
)

// Resource is the resource interface for managing a Kubernetes resource.
type Resource[T runtime.Object] interface {
	Retriever[T]
	Handler[T]
}

// ResourceImpl is the implementation of the Resource interface.
type ResourceImpl[T runtime.Object] struct {
	resource func(namespace string) Resource[T]
	handle   func(ctx context.Context, obj T) error
	base     errors.Error
}

// NewResource creates a new resource event handler.
func NewResource[T runtime.Object](
	resource func(namespace string) Resource[T],
	handle func(ctx context.Context, obj T) error,
	base errors.Error,
) Resource[T] {
	return &ResourceImpl[T]{
		resource: resource,
		handle:   handle,
		base:     base,
	}
}

// List retrieves a list of resources. Used by the retriever.
func (r *ResourceImpl[T]) List(
	ctx context.Context, options metav1.ListOptions,
) (runtime.Object, error) {
	obj, err := r.resource("").List(ctx, options)

	return obj, r.base.Wrap("failed to retrieve: %w", err)
}

// Watch watches for changes in the resource. Used by the retriever.
func (r *ResourceImpl[T]) Watch(
	ctx context.Context, options metav1.ListOptions,
) (watch.Interface, error) {
	watcher, err := r.resource("").Watch(ctx, options)

	return watcher, r.base.Wrap("failed to watch: %w", err)
}

// Handle handles the regular synchronization logic. Called by the controller.
func (r *ResourceImpl[T]) Handle(
	ctx context.Context, obj runtime.Object,
) error {
	if typed, ok := obj.(T); !ok {
		return r.base.New("invalid type [type=%T]", obj)
	} else {
		return r.handle(ctx, typed)
	}
}

// Notify notifies about errors during processing. Called by the controller.
func (r *ResourceImpl[T]) Notify(
	_ context.Context, msg string, err error,
) {
	log.WithError(err).WithFields(log.Fields{
		"handler": r.base.Error(),
	}).Error(msg)
}
