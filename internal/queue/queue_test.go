package queue_test

import (
	"context"
	"testing"

	"ollama_agent_go/internal/queue"
)

func TestInProcPublishSubscribe(t *testing.T) {
	q := queue.NewInProc()
	ctx := context.Background()

	received := make([]string, 0)
	if err := q.Subscribe(ctx, "test.topic", func(msg queue.Message) error {
		received = append(received, string(msg.Payload))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := q.Publish(ctx, queue.Message{Topic: "test.topic", Payload: []byte("hello")}); err != nil {
		t.Fatal(err)
	}
	if err := q.Publish(ctx, queue.Message{Topic: "test.topic", Payload: []byte("world")}); err != nil {
		t.Fatal(err)
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(received))
	}
	if received[0] != "hello" || received[1] != "world" {
		t.Errorf("unexpected messages: %v", received)
	}
}

func TestInProcCloseRejectsPublish(t *testing.T) {
	q := queue.NewInProc()
	_ = q.Close()
	err := q.Publish(context.Background(), queue.Message{Topic: "t", Payload: []byte("x")})
	if err == nil {
		t.Error("expected error after close, got nil")
	}
}
