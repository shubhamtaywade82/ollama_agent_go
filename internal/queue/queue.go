// Package queue provides a simple pub/sub message queue used by the agent
// runtime to publish events for downstream consumers.
package queue

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Message is one item in the queue.
type Message struct {
	Topic   string
	Payload []byte
	ID      string
	At      time.Time
}

// Publisher publishes messages to topics.
type Publisher interface {
	Publish(ctx context.Context, msg Message) error
}

// Subscriber subscribes to topics and processes messages.
type Subscriber interface {
	Subscribe(ctx context.Context, topic string, handler func(Message) error) error
	Close() error
}

// Queue is a combined publisher and subscriber.
type Queue interface {
	Publisher
	Subscriber
}

// InProcQueue is a channel-based in-process queue. Useful for single-process
// deployments and tests. For multi-process deployments, swap in a Kafka or SQS
// implementation that satisfies the same interface.
type InProcQueue struct {
	mu       sync.RWMutex
	subs     map[string][]func(Message) error
	closed   bool
	closedCh chan struct{}
}

// NewInProc creates a new InProcQueue.
func NewInProc() *InProcQueue {
	return &InProcQueue{
		subs:     make(map[string][]func(Message) error),
		closedCh: make(chan struct{}),
	}
}

// Publish delivers msg to all subscribers registered for msg.Topic.
// Returns an error if the queue is closed.
func (q *InProcQueue) Publish(_ context.Context, msg Message) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		return fmt.Errorf("queue: closed")
	}
	if msg.At.IsZero() {
		msg.At = time.Now()
	}
	for _, h := range q.subs[msg.Topic] {
		_ = h(msg) // fire-and-forget; errors are not propagated back to publisher
	}
	return nil
}

// Subscribe registers handler for topic. Messages arrive synchronously in
// the same goroutine as the publisher (in-proc behaviour).
func (q *InProcQueue) Subscribe(_ context.Context, topic string, handler func(Message) error) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return fmt.Errorf("queue: closed")
	}
	q.subs[topic] = append(q.subs[topic], handler)
	return nil
}

// Close marks the queue as closed. Subsequent Publish/Subscribe calls return errors.
func (q *InProcQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.closedCh)
	}
	return nil
}

// New returns an InProcQueue (Kafka/SQS can be added via env-var detection).
func New() Queue {
	return NewInProc()
}
