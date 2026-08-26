package controller

import (
	"context"
	"time"

	"github.com/tkrop/go-kube/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

// ErrPanic is returned when a panic occurs during event processing.
var ErrPanic = errors.New("panic")

// ResourceEventHandler is an event handler for handling resource events.
type ResourceEventHandler[T runtime.Object] struct {
	handler Handler[T]
	queue   Queue[string]
}

// NewResourceEventHandler creates a new event handler.
func NewResourceEventHandler[T runtime.Object](
	handler Handler[T],
	queue Queue[string],
) *ResourceEventHandler[T] {
	return &ResourceEventHandler[T]{
		handler: handler,
		queue:   queue,
	}
}

// OnAdd is called when an object is added.
func (r *ResourceEventHandler[T]) OnAdd(obj any, _ bool) {
	ctx := context.Background()
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		r.handler.Notify(ctx, key, err)
	} else {
		r.queue.Add(ctx, key)
	}
}

// OnUpdate is called when an object is updated.
func (r *ResourceEventHandler[T]) OnUpdate(_, obj any) {
	ctx := context.Background()
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		r.handler.Notify(ctx, key, err)
	} else {
		r.queue.Add(ctx, key)
	}
}

// OnDelete is called when an object is deleted.
func (r *ResourceEventHandler[T]) OnDelete(obj any) {
	ctx := context.Background()
	if key, err := cache.
		DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		r.handler.Notify(ctx, key, err)
	} else {
		r.queue.Add(ctx, key)
	}
}

// Processor is the default implementation of a processor.
type Processor[T runtime.Object] struct {
	handler  Handler[T]
	workers  int
	indexer  cache.Indexer
	queue    Queue[string]
	recorder Recorder
}

// NewProcessor creates a new processor. A worker count below one starts no
// workers and thereby only caches the resources without processing events.
func NewProcessor[T runtime.Object](
	handler Handler[T], informer cache.SharedIndexInformer, workers int,
	queue Queue[string], recorder Recorder,
) *Processor[T] {
	return &Processor[T]{
		handler:  handler,
		workers:  workers,
		indexer:  informer.GetIndexer(),
		queue:    queue,
		recorder: recorder,
	}
}

// Run will start the processing loop.
func (p *Processor[T]) Run(ctx context.Context) {
	defer p.queue.ShutDown(ctx)
	p.handler.Notify(ctx, "starting processor", nil)

	for range p.workers {
		go wait.Until(func() {
			p.Process(ctx)
		}, time.Second, ctx.Done())
	}

	<-ctx.Done()
	p.handler.Notify(ctx, "stopping processor", nil)
}

// Process will start a processing loop on event queue. The loop will run until
// the given context is done or the queue is shutdown.
func (p *Processor[T]) Process(ctx context.Context) {
	for {
		if p.process(ctx) {
			return
		}
	}
}

// process processes a single item from the queue. It returns true if the
// processing loop should exit. If the event handling fails or panics, the
// error is reported to the handler and the item is requeued for retry.
func (p *Processor[T]) process(ctx context.Context) bool {
	var start time.Time
	if p.recorder != nil {
		start = time.Now()
	}

	key, exit := p.queue.Get(ctx)
	if exit {
		return true
	}
	defer p.queue.Done(ctx, key)

	err := p.handle(ctx, key)
	if err == nil {
		p.queue.Forget(ctx, key)
	}

	if p.recorder != nil {
		p.recorder.DoneEvent(ctx, p.queue.Name(), err == nil, start)
	}

	return false
}

// handle looks up the resource for the given key and lets the handler process
// it. Failures are reported to the handler and returned to allow requeuing and
// recording. Panics are recovered and returned as error.
func (p *Processor[T]) handle(
	ctx context.Context, key string,
) (err error) {
	defer p.recover(ctx, key, &err)

	obj, exists, err := p.indexer.GetByKey(key)
	if err != nil {
		err = ErrController.New("get-by-key [key=%s]: %w", key, err)
		p.handler.Notify(ctx, key, err)

		return err
	} else if !exists {
		return nil
	}

	o, ok := obj.(runtime.Object)
	if !ok {
		err = ErrController.New("type assertion: %T", obj)
		p.handler.Notify(ctx, key, err)

		return err
	}

	if err = p.handler.Handle(ctx, o); err != nil {
		err = ErrController.New("handle [key=%s]: %w", key, err)
		if rerr := p.queue.Requeue(ctx, key); rerr != nil {
			p.handler.Notify(ctx, key,
				ErrController.New("could not retry: %s: %w", rerr, err))
		}

		return err
	}

	return nil
}

// recover recovers from panics during processing and notifies the handler
// about the panic error. Needs to be called using defer. If a panic occurs,
// the handler is notified with `ErrPanic` wrapped around the panic value and
// the given error is set to report the failure. This allows the handler to log
// the panic and continue processing further events.
//
// TODO: check whether this recovery is suitable in all panic cases or whether
// this behavior should be configurable to alternatively kill the operator. The
// previous behavior to just stop the processor loop for one after another
// worker is probably the least desirable behavior.
func (p *Processor[T]) recover(ctx context.Context, key string, err *error) {
	// revive:disable-next-line:defer // helper function called with defer.
	if rec := recover(); rec != nil {
		*err = ErrPanic.New(": %w", rec)
		p.handler.Notify(ctx, key, *err)
	}
}
