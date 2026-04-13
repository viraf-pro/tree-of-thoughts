package events

import (
	"sync"
	"sync/atomic"
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

func TestConcurrentPublishSubscribe(t *testing.T) {
	b := &Bus{subs: make(map[int]chan Event)}
	const numPublishers = 10
	const numEvents = 100

	id, ch := b.Subscribe()
	defer b.Unsubscribe(id)

	var wg sync.WaitGroup
	var received atomic.Int64

	// Consumer
	done := make(chan struct{})
	go func() {
		for range ch {
			received.Add(1)
		}
		close(done)
	}()

	// Concurrent publishers
	for p := 0; p < numPublishers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numEvents; i++ {
				b.Publish(Event{Type: ThoughtAdded, Timestamp: time.Now()})
			}
		}()
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond) // let consumer drain
	b.Unsubscribe(id)
	<-done

	got := received.Load()
	// Some may be dropped due to buffer overflow, but we should get at least bufSize
	if got < int64(bufSize) {
		t.Fatalf("received too few events: %d (expected at least %d)", got, bufSize)
	}
	t.Logf("received %d of %d events", got, numPublishers*numEvents)
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	b := &Bus{subs: make(map[int]chan Event)}

	var wg sync.WaitGroup
	// Concurrent subscribe/unsubscribe while publishing
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, ch := b.Subscribe()
			// Read a few events (non-blocking)
			for j := 0; j < 3; j++ {
				select {
				case <-ch:
				default:
				}
			}
			b.Unsubscribe(id)
		}()
	}

	// Publish concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			b.Publish(Event{Type: TreeCreated, Timestamp: time.Now()})
		}
	}()

	wg.Wait()
	// If we get here without deadlock or panic, the test passes
}

func TestUnsubscribeIdempotent(t *testing.T) {
	b := &Bus{subs: make(map[int]chan Event)}
	id, _ := b.Subscribe()
	b.Unsubscribe(id)
	// Second unsubscribe should not panic
	b.Unsubscribe(id)
}

func TestPublishNoSubscribers(t *testing.T) {
	b := &Bus{subs: make(map[int]chan Event)}
	// Should not panic
	b.Publish(Event{Type: TreeCreated, Timestamp: time.Now()})
}

func TestSubscriberIDs(t *testing.T) {
	b := &Bus{subs: make(map[int]chan Event)}
	id1, _ := b.Subscribe()
	id2, _ := b.Subscribe()
	id3, _ := b.Subscribe()
	defer b.Unsubscribe(id1)
	defer b.Unsubscribe(id2)
	defer b.Unsubscribe(id3)

	// IDs should be unique and sequential
	if id1 == id2 || id2 == id3 || id1 == id3 {
		t.Fatalf("IDs not unique: %d, %d, %d", id1, id2, id3)
	}
}
