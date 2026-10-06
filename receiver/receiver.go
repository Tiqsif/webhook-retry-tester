// package receiver is a fake merchant endpoint, standing in for the real
// thing a merchant would run. its only here to give the sender something
// to retry against, it can be told to fail the first few deliveries of
// every event on purpose, so tests can check the sender actually handles
// that correctly

package receiver

import (
	"net/http"
	"net/http/httptest"
	"sync"
)

// FakeReceiver tracks 2 different things on purpose, how many times a
// request actually arrived for a given event id, retries and genuine
// redeliveries both counted, and how many times it actually treated that
// event as new work. the second number should stay at 1 even if the same
// event gets delivered more than once after it already succeeded, thats
// the whole point of idempotency, a duplicate delivery shouldnt mean
// duplicate work
type FakeReceiver struct {
	mu             sync.Mutex
	failFirstN     int
	deliveryCounts map[string]int
	processed      map[string]bool
	processedCount map[string]int
	Server         *httptest.Server
}

// NewFakeReceiver starts a real local http server. it fails the first
// failFirstN requests for each distinct event id it sees, then succeeds on
// every request after that, including any later redelivery of an event it
// already succeeded on. failFirstN of 0 means it succeeds immediately
// every time, a very large failFirstN effectively means it never succeeds
func NewFakeReceiver(failFirstN int) *FakeReceiver {
	r := &FakeReceiver{
		failFirstN:     failFirstN,
		deliveryCounts: make(map[string]int),
		processed:      make(map[string]bool),
		processedCount: make(map[string]int),
	}
	r.Server = httptest.NewServer(http.HandlerFunc(r.handle))
	return r
}

func (r *FakeReceiver) handle(w http.ResponseWriter, req *http.Request) {
	eventID := req.Header.Get("X-Event-Id")

	r.mu.Lock()
	defer r.mu.Unlock()

	r.deliveryCounts[eventID]++

	// already successfully handled this exact event before, a correct
	// merchant endpoint should do exactly this, acknowledge it again
	// without redoing whatever the first delivery already did
	if r.processed[eventID] {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.deliveryCounts[eventID] <= r.failFirstN {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r.processed[eventID] = true
	r.processedCount[eventID]++
	w.WriteHeader(http.StatusOK)
}

// DeliveryCount is how many times a request for this event id has hit the
// receiver at all, retries and genuine redeliveries both count
func (r *FakeReceiver) DeliveryCount(eventID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deliveryCounts[eventID]
}

// ProcessedCount is how many times this event was actually treated as new
// work. a correct, idempotent receiver keeps this at 1 no matter how many
// times the same event id gets delivered
func (r *FakeReceiver) ProcessedCount(eventID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.processedCount[eventID]
}

// URL is the address tests point a Sender at
func (r *FakeReceiver) URL() string {
	return r.Server.URL
}

// Close shuts the fake server down, tests should defer this right after
// creating one
func (r *FakeReceiver) Close() {
	r.Server.Close()
}
