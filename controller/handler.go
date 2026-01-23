package controller

import (
	"context"
	"fmt"
	goruntime "runtime"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/tkrop/go-kube/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// StackFrames is the number of stack frames to capture.
	StackFrames = 16
	// StackBufferSize is the initial size of the buffer used to capture the
	// stack trace.
	StackBufferSize = StackFrames * 64
	// StackFramesSkip is the number of stack frames to skip when capturing a
	// stack trace. This includes the frames for capturing the stack trace and
	// to call the Stack function.
	StackFramesSkip = 5
)

// Handler is the interface the resource event handler.
type Handler[T runtime.Object] interface {
	// Handle knows how to handle resource events.
	Handle(ctx context.Context, obj runtime.Object) error
	// Notify is called to notify about errors during processing. The error
	// may be of type `ErrPanic`, to indicates that the controller loop has
	// panicked, or `nil` to indicate an informational message.
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
	logger := log.WithFields(log.Fields{
		"handler": r.base.Error(),
	})

	if errors.Is(err, ErrPanic) {
		stack := Stack(StackFramesSkip)
		logger.WithError(err).Error(msg)
		//nolint:errcheck // we want do not want to handle errors here.
		// #nosec G104 // we want do not want to handle errors here.
		log.StandardLogger().Out.Write(stack)
	} else if err != nil {
		logger.WithError(err).Error(msg)
	} else {
		logger.Trace(msg)
	}
}

// Stack returns the current stack trace, skipping the given number of frames.
func Stack(skip int) []byte {
	pc := make([]uintptr, StackFrames)
	n := goruntime.Callers(skip, pc)
	frames := goruntime.CallersFrames(pc[:n])

	builder := strings.Builder{}
	builder.Grow(StackBufferSize)

	for {
		frame, more := frames.Next()
		if frame.Function == "" {
			break
		}
		fmt.Fprintf(&builder, "%s\n\t%s:%d\n",
			frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}

	return []byte(builder.String())
}
