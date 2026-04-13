package events

import (
	"testing"
	"time"
)

func TestPublishSubscribe(t *testing.T) {
	b := &Bus{subs: make(map[int]chan Event)}

	id, ch := b.Subscribe()
	defer b.Unsubscribe(id)

	evt := Event{
		Type:      TreeCreated,
		TreeID:    "tree-1",
		Timestamp: time.Now(),
		Payload:   map[string]any{"problem": "test"},
	}
	b.Publish(evt)

	select {
	case got := <-ch:
		if got.Type != TreeCreated {
			t.Fatalf("type: got %q, want %q", got.Type, TreeCreated)
		}
		if got.TreeID != "tree-1" {
			t.Fatalf("treeId: got %q", got.TreeID)
		}
		if got.Payload["problem"] != "test" {
			t.Fatalf("payload: got %v", got.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	b := &Bus{subs: make(map[int]chan Event)}

	id1, ch1 := b.Subscribe()
	id2, ch2 := b.Subscribe()
	defer b.Unsubscribe(id1)
	defer b.Unsubscribe(id2)

	b.Publish(Event{Type: ThoughtAdded, TreeID: "t1", Timestamp: time.Now()})

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Type != ThoughtAdded {
				t.Fatalf("subscriber %d: type %q", i, got.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestUnsubscribe(t *testing.T) {
	b := &Bus{subs: make(map[int]chan Event)}

	id, ch := b.Subscribe()
	b.Unsubscribe(id)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Fatal("channel should be closed after unsubscribe")
	}

	// Publish should not panic with no subscribers
	b.Publish(Event{Type: TreeCreated, Timestamp: time.Now()})
}

func TestPublishNonBlocking(t *testing.T) {
	b := &Bus{subs: make(map[int]chan Event)}

	id, _ := b.Subscribe()
	defer b.Unsubscribe(id)

	// Fill the buffer
	for i := 0; i < bufSize+10; i++ {
		b.Publish(Event{Type: ThoughtAdded, Timestamp: time.Now()})
	}
	// Should not block — events beyond bufSize are dropped
}

func TestGetSingleton(t *testing.T) {
	b1 := Get()
	b2 := Get()
	if b1 != b2 {
		t.Fatal("Get() should return the same instance")
	}
}
