# OrbisFlow

**Distributed Financial Settlement & Resilience Infrastructure**

OrbisFlow coordinates financial transactions across multiple settlement rails
while handling concurrency, unreliable networks, liquidity constraints,
duplicate events, delayed messages, partial failures, reconciliation,
security, and recovery.

This is not a payment app. It's the infrastructure underneath financial
movement — built as one continuous system across the remaining days of the
100 Days of Code challenge, rather than a series of unrelated projects.

## scope (this scaffold)

**Architecture & Transaction Core**. What's here right now:

- Project skeleton matching the target architecture (see `docs/ARCHITECTURE.md`)
- `docker-compose.yml` bringing up Postgres, Redis, and RabbitMQ
- Schema migration (`migrations/0001_init.sql`) plus a seed script
  (`migrations/0002_seed.sql`) with three test institutions and starting
  liquidity
- A `transaction` service in Go, backed by real Postgres writes:
  - `GET /healthz`
  - `POST /v1/transactions` — validates the request, requires an
    `Idempotency-Key` header, resolves institution codes, atomically
    reserves liquidity (a conditional `UPDATE ... WHERE available >= $1`,
    which row-locks for the duration of the update), inserts the
    transaction row, and replays the same response if the same
    idempotency key is sent again

Not yet built (intentionally — see `docs/day14-notes.md` for why):
concurrent-retry dedup under real load, routing to an actual rail, outbox
events, clearing, settlement, reconciliation, the chaos lab, security
hardening, observability, load testing. Those land on their own days per
`docs/ARCHITECTURE.md`.

## Running it

```bash
cp .env.example .env
docker compose up -d
```

With `make` installed:
```bash
make migrate   # applies every file in migrations/, in order
make run       # starts the transaction service on :8110
```

Without `make` (e.g. Windows PowerShell without it installed):
```powershell
docker cp migrations/0001_init.sql orbisflow-postgres:/tmp/0001_init.sql
docker exec -it orbisflow-postgres psql -U orbisflow -d orbisflow -f /tmp/0001_init.sql
docker cp migrations/0002_seed.sql orbisflow-postgres:/tmp/0002_seed.sql
docker exec -it orbisflow-postgres psql -U orbisflow -d orbisflow -f /tmp/0002_seed.sql
go mod tidy
go run ./services/transaction
```

Test it:
```powershell
Invoke-RestMethod http://localhost:8110/healthz

Invoke-RestMethod -Uri http://localhost:8110/v1/transactions -Method Post `
  -Headers @{ "Idempotency-Key" = "test-key-001" } `
  -ContentType "application/json" `
  -Body '{"source":"institution-a","destination":"institution-b","amount":"50000","currency":"NGN","rail":"bank-transfer"}'
```
Send the same request with the same `Idempotency-Key` again — you should
get back the *same* transaction `id` instead of a new one. Send it with an
amount larger than the seeded balance and you should get a `409`.

## Ports (this machine's registry)

| Service              | Host port |
|-----------------------|-----------|
| Postgres               | 5447      |
| Redis                  | 6402      |
| RabbitMQ (AMQP)         | 5673      |
| RabbitMQ (management)   | 15673     |
| Transaction API (HTTP)  | 8110      |

Chosen to avoid every port already claimed by other running challenge
projects (5432–5436, 5445, 5446, 5544 for Postgres; 6379, 6389, 6396, 6400,
6401 for Redis; 8080, 8090, 8100 for other APIs; 9092 for finpay-redpanda;
16686 for CollabBoard's Jaeger UI).

## Project layout

See `docs/ARCHITECTURE.md` for the full target architecture and build plan.
