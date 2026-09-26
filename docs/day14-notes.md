# Day 14 notes - Transaction Core

Answers to "why" for the pieces built today, so I can explain this without
looking it up later.

**Why UUID foreign keys instead of the institution code string?**
Codes are a human-facing identifier and could theoretically be renamed;
the UUID is a stable internal reference that doesn't change even if
`institutions.code` does. It also keeps the FK narrow (16 bytes) and lets
Postgres enforce referential integrity directly instead of trusting the
application to only insert valid codes.

**Why `NUMERIC(20,2)` for amount instead of a float?**
Floats can't represent decimal currency amounts exactly (classic
0.1 + 0.2 problem). `NUMERIC` is exact, which matters the moment you start
summing or netting real money.

**Why does `outbox_events` exist already when nothing publishes to it yet?**
The table is part of the schema now so that when the Day 18 outbox worker
is built, no migration or backfill is needed - any code that starts writing
outbox rows inside the same transaction as a business write will just work.

**Why reserve liquidity with a single conditional `UPDATE ... WHERE
available >= $1 RETURNING available` instead of `SELECT ... FOR UPDATE`
then a separate `UPDATE`?**
Both take a row lock, but the single-statement version means there's no
window between reading the balance and writing the new one - the check and
the deduction happen as one atomic operation. Two concurrent requests
against the same account can't both read "sufficient balance" before
either writes, because the second one blocks on the row lock until the
first's UPDATE (and its WHERE-clause re-evaluation) completes. Proving this
holds under real concurrent load (many goroutines, not just reasoning
about it) is Day 15's job.

**Why require `Idempotency-Key` as a header instead of a body field?**
That's how the original spec described it (`Idempotency-Key: 7d8f...`),
and it matches how idempotency keys are conventionally sent (Stripe does
the same) - it's metadata about the request, not part of the business
payload.

**What's still missing (intentionally, for later days)?**
- No dedup on *concurrent* identical retries yet - two requests with the
  same key arriving at the same instant could both pass the "does this key
  exist" check before either commits. Closing that gap (via the unique
  constraint on `idempotency_key` plus handling the resulting DB error as
  a 409/200) is Day 16.
- No routing to an actual rail - `rail` is currently just stored as a
  string. Day 17.
- No outbox event is written on transaction creation yet. Day 18.
