# OrbisFlow Architecture

## The problem

Multiple institutions connect through OrbisFlow across different rails
(bank transfer, payment rail A/B). OrbisFlow must route, clear, and settle
transactions correctly despite: network timeouts, duplicate/out-of-order
messages, competing liquidity claims, partial failures, ambiguous external
responses, reconciliation mismatches, replay attacks, and load spikes.

## Components

```
API Gateway
     |
Transaction Core
     |
  +--+--+--------+
  |     |        |
Routing Risk  Idempotency
  |
Clearing Engine
  |
Settlement Engine
  |         \
Institution A  Institution B
  |
Message Broker (RabbitMQ)
  |
  +---------+----------+
  |         |          |
Reconciliation Recovery Notifications
  |
Discrepancy Engine
```

## Settlement states

```
CREATED -> VALIDATING -> CLEARED -> SETTLEMENT_PENDING -> PROCESSING
                                                           |-> SUCCESS
                                                           |-> FAILED
                                                           |-> UNKNOWN
```

`UNKNOWN` exists because a network failure after sending a settlement
instruction does not tell us whether it succeeded externally — that's
resolved by reconciliation, not assumed.

## Reconciliation states

`MATCHED`, `MISSING_INTERNAL`, `MISSING_EXTERNAL`, `AMOUNT_MISMATCH`,
`DUPLICATE`, `UNKNOWN`.

## Stack

Go, PostgreSQL + SQLC, Redis, RabbitMQ (transactional outbox pattern),
Docker, OpenTelemetry, Prometheus, Grafana, k6, GitHub Actions,
Testcontainers. Kubernetes deferred until there are enough services to
justify it.

## Build plan (Day 14 -> Day 22)

| Day | Focus |
|-----|-------|
| 14  | Architecture & Transaction Core |
| 15  | PostgreSQL + Concurrency (reproduce the race condition on purpose) |
| 16  | Idempotency |
| 17  | Routing & simulated external rails |
| 18  | RabbitMQ + Outbox pattern |
| 19  | Clearing & Settlement |
| 20  | Reconciliation & Recovery |
| 21  | Security + Observability + Reliability (HMAC, replay protection, rate limiting, tracing, metrics, circuit breakers) |
| 22  | Chaos + Load testing (Fault Lab, k6) |

## Explicit non-goals

Not a copy of any existing employer's ledger architecture or ADR. No
accounting-theory rabbit hole — only the double-entry/netting concepts the
system actually needs. No Kubernetes, Kafka, or extra tech until a
component's own requirements justify it.

## What "done" looks like

Being able to explain, with evidence (code, tests, traces, metrics, load
test results): why each lock/transaction boundary exists, how duplicate
financial operations are prevented, what happens when a message is
delivered twice or a consumer crashes mid-processing, why `UNKNOWN` exists
and how it's resolved, how tenant isolation is enforced, and how the system
was deliberately broken and recovered.
