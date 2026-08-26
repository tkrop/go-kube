package controller

import (
	"context"
	"fmt"

	"github.com/tkrop/go-kube/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
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
type retriever[T runtime.Object] struct {
	// The underlying retriever interface.
	iface Retriever[T]
	// The base error for wrapping errors.
	base *errors.Error
}

// NewRetriever creates a new retriever adapter for given retriever interface.
func NewRetriever[T runtime.Object](
	iface Retriever[T], base *errors.Error,
) Retriever[T] {
	return &retriever[T]{
		iface: iface,
		base:  base,
	}
}

// List retrieves a list of resources from the API server.
func (a *retriever[T]) List(
	ctx context.Context, opts metav1.ListOptions,
) (T, error) {
	list, err := a.iface.List(ctx, opts)

	return list, a.base.Wrap("listing: %w", err)
}

// Watch starts watching for changes on the resources from the API server.
func (a *retriever[T]) Watch(
	ctx context.Context, opts metav1.ListOptions,
) (watch.Interface, error) {
	watch, err := a.iface.Watch(ctx, opts)

	return watch, a.base.Wrap("watching: %w", err)
}

// filterRetriever is a retriever narrowing the observed resources by selectors.
type filterRetriever[T runtime.Object] struct {
	// The underlying retriever interface.
	iface Retriever[T]
	// The base error for wrapping errors.
	*errors.Error
	// The labels selector string applied to the list and watch requests.
	labels string
	// The fields selector string applied to the list and watch requests.
	fields string
}

// NewFilterRetriever creates a retriever narrowing the resources observed
// by the given retriever applying the given label and field selector on the
// initial list as well as on the watch requests.
func NewFilterRetriever[T runtime.Object](
	iface Retriever[T], base *errors.Error,
	labels labels.Selector, fields fields.Selector,
) Retriever[T] {
	return &filterRetriever[T]{
		iface:  iface,
		Error:  base,
		labels: value(labels),
		fields: value(fields),
	}
}

// value returns the string representation of the given value or an empty
// string if the value is nil.
func value(value fmt.Stringer) string {
	if value == nil {
		return ""
	}

	return value.String()
}

// List retrieves the selected resources from the API server.
func (f *filterRetriever[T]) List(
	ctx context.Context, opts metav1.ListOptions,
) (T, error) {
	list, err := f.iface.List(ctx, f.options(opts))

	return list, f.Wrap("listing: %w", err)
}

// Watch starts watching for changes on the selected resources from the API
// server.
func (f *filterRetriever[T]) Watch(
	ctx context.Context, opts metav1.ListOptions,
) (watch.Interface, error) {
	watch, err := f.iface.Watch(ctx, f.options(opts))

	return watch, f.Wrap("watching: %w", err)
}

// options applies the configured selectors to the given list options.
func (f *filterRetriever[T]) options(
	opts metav1.ListOptions,
) metav1.ListOptions {
	opts.LabelSelector = f.labels
	opts.FieldSelector = f.fields

	return opts
}
