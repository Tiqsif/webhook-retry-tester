// package sender contains the retry logic thats actually under test. it
// doesnt know anything about payments specifically, just how to deliver
// one event to one url, retrying with backoff if it doesnt get a success
// response back

package sender

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// Event is one thing were telling a merchant about, a payment succeeding,
// failing, whatever. ID has to stay the same across every retry of the
// same event, thats what lets a receiver on the other end tell a retry
// apart from a brand new event
type Event struct {
	ID      string
	Type    string
	Payload map[string]any
}

// Config controls how a Sender retries. BaseDelay doubles on every
// attempt, so attempt 1 waits BaseDelay before attempt 2, attempt 2 waits
// BaseDelay*2 before attempt 3, and so on, up to MaxAttempts total tries
type Config struct {
	URL         string
	MaxAttempts int
	BaseDelay   time.Duration
	Client      *http.Client
}

// Result is what actually happened trying to deliver one event.
// AttemptTimes records when each try actually fired, so tests can check
// the backoff really grew instead of just trusting that it did
type Result struct {
	Succeeded    bool
	Attempts     int
	AttemptTimes []time.Time
}

type Sender struct {
	cfg Config
}

func New(cfg Config) *Sender {
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	return &Sender{cfg: cfg}
}

// Deliver posts the event to cfg.URL, retrying with exponential backoff on
// anything that isnt a 2xx, up to cfg.MaxAttempts tries total. the same
// event id goes out on every single attempt, on purpose, thats what makes
// a receiver able to recognize a retry for what it is
func (s *Sender) Deliver(event Event) Result {
	var result Result

	body, err := json.Marshal(event.Payload)
	if err != nil {
		// a bad payload isnt something retrying will ever fix, so this
		// doesnt even attempt once
		return result
	}

	for attempt := 1; attempt <= s.cfg.MaxAttempts; attempt++ {
		result.AttemptTimes = append(result.AttemptTimes, time.Now())
		result.Attempts = attempt

		req, err := http.NewRequest(http.MethodPost, s.cfg.URL, bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Event-Id", event.ID)
			req.Header.Set("X-Event-Type", event.Type)

			resp, err := s.cfg.Client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					result.Succeeded = true
					return result
				}
			}
		}

		if attempt < s.cfg.MaxAttempts {
			delay := s.cfg.BaseDelay * time.Duration(1<<uint(attempt-1))
			time.Sleep(delay)
		}
	}

	return result
}
