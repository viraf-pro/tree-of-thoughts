package events

import (
	"log"
	"sync"
)

const bufSize = 64

// Bus is a typed pub/sub event bus. Safe for concurrent use.
type Bus struct {
	mu   sync.RWMutex
	subs map[int]chan Event
	next int
}

var (
	global     *Bus
	globalOnce sync.Once
)

// Get returns the global event bus singleton.
func Get() *Bus {
	globalOnce.Do(func() {
		global = &Bus{subs: make(map[int]chan Event)}
	})
	return global
}

// Subscribe returns a buffered channel of events and an ID for unsubscribing.
func (b *Bus) Subscribe() (int, <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, bufSize)
	b.subs[id] = ch
	return id, ch
}

// Unsubscribe removes a subscription and closes its channel.
func (b *Bus) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subs[id]; ok {
		delete(b.subs, id)
		close(ch)
	}
}

// Publish fans out an event to all subscribers.
// Non-blocking: if a subscriber's buffer is full, the event is dropped
// for that subscriber (logged once).
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
			log.Printf("events: dropped %s for slow subscriber", e.Type)
		}
	}
}
