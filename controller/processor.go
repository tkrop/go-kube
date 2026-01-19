package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

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

// NewProcessor creates a new processor.
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

	obj, exists, err := p.indexer.GetByKey(key)
	if err != nil {
		p.handler.Notify(ctx, key, err)

		return false
	} else if !exists {
		return false
	}

	defer p.recover(ctx, key)

	if o, ok := obj.(runtime.Object); !ok {
		p.handler.Notify(ctx, key,
			ErrController.New("type assertion: %T", obj))
	} else if err = p.handler.Handle(ctx, o); err != nil {
		if rerr := p.queue.Requeue(ctx, key); rerr != nil {
			p.handler.Notify(ctx, key,
				ErrController.New("could not retry: %s: %w", rerr, err))
		}
	}

	if p.recorder != nil {
		p.recorder.DoneEvent(ctx, p.queue.Name(), err == nil, start)
	}

	return false
}

// recover recovers from panics during processing and notifies the handler
// about the panic error. Needs to be called using defer.
func (p *Processor[T]) recover(ctx context.Context, key string) {
	// revive:disable-next-line:defer // helper function called with defer.
	if err := recover(); err != nil {
		p.handler.Notify(ctx, key, ErrController.New("panic: %v", err))
	}
}
