// small standalone program you can just run to watch the retry behavior
// happen in real time, no need to read or write any test code first

package main

import (
	"fmt"
	"time"

	"github.com/Tiqsif/webhook-retry-tester/receiver"
	"github.com/Tiqsif/webhook-retry-tester/sender"
)

func main() {
	// this fake receiver fails the first 2 deliveries of any event, then
	// succeeds from the 3rd delivery onward, standing in for a merchants
	// endpoint that was briefly down or erroring
	r := receiver.NewFakeReceiver(2)
	defer r.Close()

	s := sender.New(sender.Config{
		URL:         r.URL(),
		MaxAttempts: 5,
		BaseDelay:   200 * time.Millisecond,
	})

	event := sender.Event{
		ID:      "evt_demo_1",
		Type:    "payment.succeeded",
		Payload: map[string]any{"amount": 29.99, "currency": "usd"},
	}

	fmt.Printf("delivering event %s to a receiver that fails the first 2 tries...\n\n", event.ID)
	result := s.Deliver(event)

	fmt.Printf("succeeded: %v, attempts: %d\n", result.Succeeded, result.Attempts)
	for i, at := range result.AttemptTimes {
		fmt.Printf("  attempt %d at %s\n", i+1, at.Format("15:04:05.000"))
	}

	fmt.Println("\nnow redelivering the exact same event id, simulating a duplicate...")
	s.Deliver(event)
	fmt.Printf("receiver saw %d total deliveries for this event, but only processed it %d time(s)\n",
		r.DeliveryCount(event.ID), r.ProcessedCount(event.ID))
}
