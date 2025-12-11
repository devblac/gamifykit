package engine

import (
	"context"
	"sync"
	"time"

	"gamifykit/core"
)

type DispatchMode int

const (
	DispatchSync DispatchMode = iota
	DispatchAsync
)

type subscription struct {
	id  int64
	typ core.EventType
	fn  func(context.Context, core.Event)
}

// EventBus provides thread-safe pub/sub with sync and async dispatch.
type EventBus struct {
	mode         DispatchMode
	mu           sync.RWMutex
	subs         map[core.EventType]map[int64]subscription
	nextID       int64
	asyncQueue   chan core.Event
	asyncWorkers int
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewEventBus(mode DispatchMode) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	eb := &EventBus{
		mode:         mode,
		subs:         make(map[core.EventType]map[int64]subscription),
		asyncQueue:   make(chan core.Event, 2048),
		asyncWorkers: 4,
		ctx:          ctx,
		cancel:       cancel,
	}
	if mode == DispatchAsync {
		eb.startWorkers()
	}
	return eb
}

func (e *EventBus) startWorkers() {
	for i := 0; i < e.asyncWorkers; i++ {
		go func() {
			for {
				select {
				case ev := <-e.asyncQueue:
					e.dispatchSync(context.Background(), ev)
				case <-e.ctx.Done():
					return
				}
			}
		}()
	}
}

// Close stops async workers.
func (e *EventBus) Close() {
	e.cancel()
	// allow workers to drain briefly
	time.Sleep(10 * time.Millisecond)
}

// Subscribe registers a handler for an event type. Returns unsubscribe func.
func (e *EventBus) Subscribe(typ core.EventType, handler func(context.Context, core.Event)) func() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nextID++
	id := e.nextID
	if e.subs[typ] == nil {
		e.subs[typ] = make(map[int64]subscription)
	}
	e.subs[typ][id] = subscription{id: id, typ: typ, fn: handler}
	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if m := e.subs[typ]; m != nil {
			delete(m, id)
		}
	}
}

// Publish sends an event to subscribers.
func (e *EventBus) Publish(ctx context.Context, ev core.Event) {
	if e.mode == DispatchAsync {
		select {
		case e.asyncQueue <- ev:
		default:
			// Drop if queue full to preserve latency; alternative is blocking
		}
		return
	}
	e.dispatchSync(ctx, ev)
}

func (e *EventBus) dispatchSync(ctx context.Context, ev core.Event) {
	e.mu.RLock()
	subs := e.subs[ev.Type]
	// copy to avoid holding lock during callbacks
	handlers := make([]func(context.Context, core.Event), 0, len(subs))
	for _, s := range subs {
		handlers = append(handlers, s.fn)
	}
	e.mu.RUnlock()
	for _, h := range handlers {
		h(ctx, ev)
	}
}
