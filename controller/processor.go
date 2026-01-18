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
		go func() {
			wait.Until(p.Process, time.Second, ctx.Done())
		}()
	}
}

// Process will start a processing loop on event queue.
func (p *Processor[T]) Process() {
	var start time.Time
	ctx := context.Background()

	for {
		key, exit := p.queue.Get(ctx)
		if exit {
			return
		}

		if p.recorder != nil {
			start = time.Now()
		}

		obj, exists, err := p.indexer.GetByKey(key)
		if err != nil {
			p.handler.Notify(ctx, key, err)

			continue
		} else if !exists {
			continue
		}

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
	}
}
