package controller

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/tkrop/go-kube/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

// Handler is the interface the resource event handler.
type Handler[T runtime.Object] interface {
	// Handle knows how to handle resource events.
	Handle(ctx context.Context, obj runtime.Object) error
	// Notify is called to notify about errors during processing.
	Notify(ctx context.Context, key string, err error)
}

// handler is the implementation of the resource event handler.
type handler[T runtime.Object] struct {
	handle func(ctx context.Context, obj T) error
	base   *errors.Error
}

// NewHandler creates a new resource event handler.
func NewHandler[T runtime.Object](
	handle func(ctx context.Context, obj T) error,
	base *errors.Error,
) Handler[T] {
	return &handler[T]{
		handle: handle,
		base:   base,
	}
}

// Handle handles the regular synchronization logic. Called by the controller.
func (r *handler[T]) Handle(
	ctx context.Context, obj runtime.Object,
) error {
	if typed, ok := obj.(T); !ok {
		return r.base.New("invalid type [type=%T]", obj)
	} else {
		return r.handle(ctx, typed)
	}
}

// Notify notifies about errors during processing. Called by the controller.
func (r *handler[T]) Notify(
	_ context.Context, msg string, err error,
) {
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"handler": r.base.Error(),
		}).Error(msg)
	} else {
		log.WithFields(log.Fields{
			"handler": r.base.Error(),
		}).Trace(msg)
	}
}
