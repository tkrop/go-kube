package controller

import (
	"context"

	"github.com/tkrop/go-kube/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// Retriever is the retriever interface for the service handling a resource.
type Retriever[T runtime.Object] interface {
	// List retrieves the list of resources from the API server.
	List(ctx context.Context, options metav1.ListOptions) (T, error)
	// Watch starts watching for changes on the resources from the API server.
	Watch(
		ctx context.Context, options metav1.ListOptions,
	) (watch.Interface, error)
}

// retriever is the implementation of the retriever interface.
type retriever[L runtime.Object] struct {
	iface Retriever[L]
	base  *errors.Error
}

// NewRetriever creates a new retriever adapter for given retriever interface.
func NewRetriever[L runtime.Object](
	iface Retriever[L], base *errors.Error,
) Retriever[L] {
	return &retriever[L]{
		iface: iface,
		base:  base,
	}
}

// List retrieves a list of resources from the API server.
func (a *retriever[L]) List(
	ctx context.Context, opts metav1.ListOptions,
) (L, error) {
	list, err := a.iface.List(ctx, opts)

	return list, a.base.Wrap("listing: %w", err)
}

// Watch starts watching for changes on the resources from the API server.
func (a *retriever[L]) Watch(
	ctx context.Context, opts metav1.ListOptions,
) (watch.Interface, error) {
	watch, err := a.iface.Watch(ctx, opts)

	return watch, a.base.Wrap("watching: %w", err)
}
