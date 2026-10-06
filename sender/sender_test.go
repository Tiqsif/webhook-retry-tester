package sender_test

import (
	"testing"
	"time"

	"github.com/Tiqsif/webhook-retry-tester/receiver"
	"github.com/Tiqsif/webhook-retry-tester/sender"
)

func TestDeliver_SucceedsFirstTry(t *testing.T) {
	r := receiver.NewFakeReceiver(0) // never fails
	defer r.Close()

	s := sender.New(sender.Config{
		URL:         r.URL(),
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
	})

	result := s.Deliver(sender.Event{
		ID:      "evt_1",
		Type:    "payment.succeeded",
		Payload: map[string]any{"amount": 29.99},
	})

	if !result.Succeeded {
		t.Fatalf("expected delivery to succeed, got failure after %d attempts", result.Attempts)
	}
	if result.Attempts != 1 {
		t.Fatalf("expected exactly 1 attempt when the receiver never fails, got %d", result.Attempts)
	}
}

func TestDeliver_RetriesThenSucceeds(t *testing.T) {
	r := receiver.NewFakeReceiver(2) // fails the first 2 tries, succeeds on the 3rd
	defer r.Close()

	s := sender.New(sender.Config{
		URL:         r.URL(),
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
	})

	result := s.Deliver(sender.Event{
		ID:      "evt_2",
		Type:    "payment.succeeded",
		Payload: map[string]any{"amount": 29.99},
	})

	if !result.Succeeded {
		t.Fatalf("expected delivery to eventually succeed, got failure after %d attempts", result.Attempts)
	}
	if result.Attempts != 3 {
		t.Fatalf("expected exactly 3 attempts, 2 failures then a success, got %d", result.Attempts)
	}
	if got := r.DeliveryCount("evt_2"); got != 3 {
		t.Fatalf("expected the receiver to have seen 3 deliveries for this event, saw %d", got)
	}
}

func TestDeliver_GivesUpAfterMaxAttempts(t *testing.T) {
	r := receiver.NewFakeReceiver(999) // always fails
	defer r.Close()

	s := sender.New(sender.Config{
		URL:         r.URL(),
		MaxAttempts: 4,
		BaseDelay:   5 * time.Millisecond,
	})

	result := s.Deliver(sender.Event{
		ID:      "evt_3",
		Type:    "payment.failed",
		Payload: map[string]any{"amount": 29.99},
	})

	if result.Succeeded {
		t.Fatalf("expected delivery to fail, a receiver that always fails should never succeed")
	}
	if result.Attempts != 4 {
		t.Fatalf("expected the sender to stop at exactly 4 attempts, got %d", result.Attempts)
	}
}

func TestDeliver_BackoffGrowsBetweenAttempts(t *testing.T) {
	r := receiver.NewFakeReceiver(999) // always fails, so we get every attempt
	defer r.Close()

	base := 20 * time.Millisecond
	s := sender.New(sender.Config{
		URL:         r.URL(),
		MaxAttempts: 3,
		BaseDelay:   base,
	})

	result := s.Deliver(sender.Event{
		ID:      "evt_4",
		Type:    "payment.failed",
		Payload: map[string]any{},
	})

	if len(result.AttemptTimes) != 3 {
		t.Fatalf("expected 3 recorded attempts, got %d", len(result.AttemptTimes))
	}

	firstGap := result.AttemptTimes[1].Sub(result.AttemptTimes[0])
	secondGap := result.AttemptTimes[2].Sub(result.AttemptTimes[1])

	// second gap should be roughly double the first, not exact since real
	// timing always has some slack, this just checks it actually grew and
	// isnt firing instantly back to back
	if secondGap < firstGap {
		t.Fatalf("expected backoff to grow between attempts, first gap %v, second gap %v", firstGap, secondGap)
	}
	if firstGap < base {
		t.Fatalf("expected the first retry to wait at least the base delay of %v, only waited %v", base, firstGap)
	}
}

func TestReceiver_OnlyProcessesEventOnce(t *testing.T) {
	// this one tests the receiver side directly, does it actually know
	// the difference between a fresh event and a repeat of one it already
	// succeeded on
	r := receiver.NewFakeReceiver(0)
	defer r.Close()

	s := sender.New(sender.Config{
		URL:         r.URL(),
		MaxAttempts: 1,
		BaseDelay:   time.Millisecond,
	})

	event := sender.Event{
		ID:      "evt_5",
		Type:    "payment.succeeded",
		Payload: map[string]any{},
	}
	s.Deliver(event)
	s.Deliver(event) // a genuine redelivery of the same event, not a retry

	if got := r.DeliveryCount("evt_5"); got != 2 {
		t.Fatalf("expected 2 deliveries to have arrived, got %d", got)
	}
	if got := r.ProcessedCount("evt_5"); got != 1 {
		t.Fatalf("expected the event to only be processed once despite 2 deliveries, got %d", got)
	}
}
