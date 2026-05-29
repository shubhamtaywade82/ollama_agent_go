// Package runtime implements the orchestration layer between the UI and the
// agent. It manages session lifecycle, policy enforcement, storage, and
// observability without being tied to any specific UI framework.
package runtime

import "sync"

// BusEvent is an event published to the EventBus.
type BusEvent struct {
	Name    string
	Payload any
}

// Handler processes a BusEvent.
type Handler func(BusEvent)

// EventBus provides publish/subscribe messaging between runtime components.
type EventBus interface {
	Publish(e BusEvent)
	Subscribe(name string, h Handler)
}

// InProcBus is an in-process, synchronous EventBus.
type InProcBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewInProcBus creates an empty InProcBus.
func NewInProcBus() *InProcBus {
	return &InProcBus{handlers: make(map[string][]Handler)}
}

// Subscribe registers a handler for the given event name.
func (b *InProcBus) Subscribe(name string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = append(b.handlers[name], h)
}

// Publish calls all handlers registered for e.Name synchronously.
func (b *InProcBus) Publish(e BusEvent) {
	b.mu.RLock()
	hs := b.handlers[e.Name]
	b.mu.RUnlock()
	for _, h := range hs {
		h(e)
	}
}
