# webhook-retry-tester

a go project testing how reliably a webhook gets delivered when the
receiving end isnt cooperating. payment gateways lean heavily on webhooks
to tell a merchant a charge succeeded, failed, or got refunded, and
getting that delivery right, retrying when the merchants endpoint is down
without accidentally telling them twice, is a real, specific problem in
that industry, not just a generic networking exercise.

## what this is showing

a sender that retries a failed delivery with exponential backoff instead
of giving up immediately or hammering the same url in a tight loop, and a
receiver that only treats an event as new work once, even if the exact
same event gets delivered to it more than once. both are plain go, no
external framework, tested with gos own `testing` package.

```
webhook-retry-tester/
├── .github/workflows/test.yml   # ci, go vet and go test on every push
├── sender/
│   ├── sender.go                 # the retry logic under test
│   └── sender_test.go
├── receiver/
│   └── receiver.go               # fake merchant endpoint, fails on purpose when told to
├── cmd/demo/main.go              # a small runnable example, no test code needed to see it work
└── go.mod
```

## the sender

`sender.Deliver()` posts an event to a url. if it doesnt get back a 2xx,
it waits and tries again, doubling the wait each time, up to a configured
max number of attempts, then gives up and reports failure. the same event
id goes out with every single attempt, in an `X-Event-Id` header, thats
what lets a receiver on the other end tell "this is attempt 3 of the same
event" apart from "this is a new event."

## the receiver

`receiver.FakeReceiver` is a real local http server, started with gos own
`httptest` package, standing in for a merchants endpoint. it can be told
to fail the first n deliveries of any event before finally succeeding,
simulating an endpoint that was briefly down or erroring. it also tracks
2 different numbers on purpose, `DeliveryCount`, how many requests
actually arrived for an event, retries and genuine redeliveries both
counted, and `ProcessedCount`, how many times it actually treated that
event as new work. a correct receiver keeps `ProcessedCount` at 1 no
matter how many times the same event shows up, thats the actual meaning
of idempotency, not just a word for "handles retries."

## the tests

`sender/sender_test.go` covers: a delivery that succeeds on the first try
does exactly 1 attempt, a receiver that fails twice then succeeds gets
exactly 3 attempts before the sender reports success, a receiver that
always fails makes the sender stop at its configured max attempts instead
of retrying forever, the wait between attempts actually grows instead of
firing back to back, and delivering the exact same event twice on purpose
still only gets processed once.

## running it

```bash
go test ./... -v
```

or watch the retry and idempotency behavior happen in real time without
reading any test code:

```bash
go run ./cmd/demo
```

## ci

`.github/workflows/test.yml` runs `go vet` and `go test ./... -v` on
every push. no separate service needs to boot first, the fake receiver
lives entirely inside the test process itself, so theres nothing to wait
on being ready before the tests can start.
