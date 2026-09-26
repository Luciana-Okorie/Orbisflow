# OrbisFlow Architecture

## Overview

OrbisFlow is a backend infrastructure system for processing transactions
across multiple financial rails while maintaining correctness under
concurrency, retries, failures, and distributed processing.

The system is being built incrementally.

Day 14 establishes the transaction foundation: request validation,
institution resolution, PostgreSQL transactions, liquidity reservation,
idempotency, and persistent transaction state.

Future stages will extend this foundation with routing, simulated external
rails, asynchronous messaging, clearing, settlement, reconciliation,
security hardening, observability, and fault testing.

---

# Day 14 Architecture

## Current Implementation

```text
                    Client
                      |
                      | HTTP
                      v
            +----------------------+
            |   Transaction API    |
            |      Go / net/http   |
            +----------+-----------+
                       |
                       v
            +----------------------+
            |   Transaction Core   |
            +----------+-----------+
                       |
          +------------+------------+
          |                         |
          v                         v
   Idempotency Check        Institution Lookup
          |                         |
          +------------+------------+
                       |
                       v
            +----------------------+
            | PostgreSQL Transaction|
            |                      |
            |  1. Reserve Liquidity|
            |  2. Create Transaction|
            |  3. Commit / Rollback|
            +----------+-----------+
                       |
                       v
            +----------------------+
            |  Transaction State   |
            |      CREATED         |
            +----------------------+

The Day 14 implementation intentionally keeps the architecture small.

Only the transaction core and its PostgreSQL persistence layer are active
today.

Redis, RabbitMQ, routing, clearing, settlement, reconciliation,
observability, and fault-injection components are part of the planned
architecture and will be introduced when their respective engineering
problems are implemented.

Request Flow

A transaction begins with:

POST /v1/transactions

The request contains:

{
  "source": "institution-a",
  "destination": "institution-b",
  "amount": "50000",
  "currency": "NGN",
  "rail": "bank-transfer"
}

An Idempotency-Key is required as an HTTP header:

Idempotency-Key: test-key-001

The request then follows this flow:

Client
  |
  v
HTTP Request
  |
  v
Validate Request
  |
  v
Check Idempotency Key
  |
  +---- existing transaction ----> Return existing transaction
  |
  v
Begin PostgreSQL Transaction
  |
  v
Resolve Source Institution
  |
  v
Resolve Destination Institution
  |
  v
Reserve Liquidity
  |
  v
Create Transaction
  |
  v
Commit
  |
  v
Return Transaction

If an operation fails before commit, the PostgreSQL transaction is rolled
back.

Liquidity Reservation

The most important correctness operation implemented on Day 14 is the
atomic liquidity reservation.

The system uses a conditional database update:

UPDATE liquidity_accounts
SET available = available - $1,
    updated_at = now()
WHERE institution_id = $2
  AND currency = $3
  AND available >= $1
RETURNING available;

The database performs the availability check and deduction as one operation.

This prevents the application from following an unsafe pattern such as:

Read balance
     |
Check balance
     |
Update balance

Instead, PostgreSQL performs the condition and state change together.

Conceptually:

             Available Liquidity
                    |
                    v
          +-------------------+
          | available >=      |
          | requested amount? |
          +---------+---------+
                    |
             +------+------+
             |             |
            YES            NO
             |             |
             v             v
      Reserve amount      Reject
             |
             v
       Continue transaction

The mechanism is designed to prevent two concurrent operations from both
successfully reserving the same available liquidity.

Day 15 will verify this behaviour experimentally with real concurrent
requests.

Database Transaction Boundary

Liquidity reservation and transaction creation occur inside the same
PostgreSQL transaction.

BEGIN
  |
  +-- Resolve source institution
  |
  +-- Resolve destination institution
  |
  +-- Reserve liquidity
  |
  +-- Insert transaction
  |
COMMIT

If any step fails:

BEGIN
  |
  +-- Reserve liquidity
  |
  +-- Transaction insert fails
  |
ROLLBACK
  |
  v
Liquidity reservation is reverted

This prevents the system from permanently reserving liquidity when the
transaction record itself was not successfully created.

Data Model

Day 14 currently uses three primary tables.

Institutions

Represents institutions participating in the system.

institutions
-------------
id
code
name
created_at

code is unique and is used by the API as the human-facing identifier.

The database uses the institution UUID internally for relationships.

Liquidity Accounts

Represents available liquidity belonging to an institution in a currency.

liquidity_accounts
------------------
id
institution_id
currency
available
updated_at

Each institution can have one liquidity account per currency.

The database enforces this with:

UNIQUE (institution_id, currency)
Transactions

Stores transaction requests accepted by the transaction core.

transactions
------------
id
idempotency_key
source_id
destination_id
amount
currency
rail
status
created_at
updated_at

Important database constraints include:

idempotency_key → UNIQUE
amount          → CHECK (amount > 0)
source_id       → FOREIGN KEY
destination_id  → FOREIGN KEY

Transaction amounts use:

NUMERIC(20,2)

rather than floating-point values so that monetary amounts are stored with
exact decimal precision.

Transaction States

The database defines the transaction lifecycle states that will be used
throughout the project:

CREATED
VALIDATING
CLEARED
SETTLEMENT_PENDING
PROCESSING
SUCCESS
FAILED
UNKNOWN

Day 14 currently creates transactions in:

CREATED

The remaining states will become active as the corresponding transaction
processing stages are implemented.

The UNKNOWN state is important for future external-rail processing.

If an external operation is sent successfully but the response is lost
because of a network failure, the system cannot safely assume that the
operation failed.

That state will later be resolved through recovery and reconciliation.

Idempotency

Day 14 introduces basic idempotent request handling.

Every transaction request requires:

Idempotency-Key

The key is stored with the transaction and enforced as unique by
PostgreSQL.

When the same key is received after a transaction already exists, OrbisFlow
returns the existing transaction instead of creating another one.

Request
  |
  v
Idempotency Key
  |
  +---- Exists ----> Return existing transaction
  |
  +---- New --------> Continue processing

Full protection against two identical requests arriving concurrently and
both passing the initial lookup is intentionally deferred to the dedicated
idempotency stage.

Monetary Data

OrbisFlow does not use floating-point numbers for transaction amounts.

Amounts are represented as decimal values and stored in PostgreSQL using:

NUMERIC(20,2)

This provides exact decimal storage for the current currency model.

Future stages may extend the monetary model if multi-currency settlement
and netting require additional rules.

Current Technology Stack
Go

Used for the transaction service and backend implementation.

The current HTTP layer uses Go's standard net/http package.

PostgreSQL

Used as the source of persistent transaction and liquidity state.

PostgreSQL currently handles:

transaction persistence
liquidity reservation
constraints
foreign keys
unique idempotency keys
transaction boundaries
atomic state changes
pgx

The Go service uses pgx and pgxpool to communicate with PostgreSQL.

This provides direct control over:

database transactions
SQL queries
connection pooling
row-level database operations
Docker

Docker Compose provides the local infrastructure environment.

Current containers:

PostgreSQL
Redis
RabbitMQ

PostgreSQL is actively used by Day 14.

Redis and RabbitMQ are provisioned for later stages.

Planned Architecture

As OrbisFlow develops, the transaction core will expand into a distributed
processing pipeline.

                         Client
                           |
                           v
                 +-------------------+
                 |   Transaction API |
                 +---------+---------+
                           |
                           v
                 +-------------------+
                 | Transaction Core  |
                 +---------+---------+
                           |
             +-------------+-------------+
             |             |             |
             v             v             v
          Routing         Risk       Idempotency
             |
             v
      +----------------+
      | Clearing       |
      | Engine         |
      +-------+--------+
              |
              v
      +----------------+
      | Settlement     |
      | Engine         |
      +-------+--------+
              |
              v
       External Rails
              |
              v
      +----------------+
      | Recovery /     |
      | Reconciliation |
      +-------+--------+
              |
              v
       PostgreSQL

Asynchronous processing will later be introduced through an outbox and
message broker:

Transaction Core
      |
      v
 PostgreSQL
      |
      +---- Transaction
      |
      +---- Outbox Event
                 |
                 v
             Publisher
                 |
                 v
             RabbitMQ
                 |
        +--------+--------+
        |        |        |
        v        v        v
     Clearing Settlement Recovery
      Worker    Worker     Worker

These components are planned and are not part of the Day 14
implementation.

Future Failure Scenarios

OrbisFlow is intentionally being designed around failure rather than only
the successful path.

Future stages will simulate:

Provider timeout
Provider unavailable
Duplicate request
Duplicate message
Delayed message
Out-of-order message
Consumer crash
Partial processing
Unknown external result
Settlement mismatch
Concurrent operations
Replay attack
Traffic spikes

The goal is to make failure behaviour explicit and testable.

Engineering Principles
1. Database-enforced correctness

Important invariants should be enforced by PostgreSQL where possible rather
than relying entirely on application code.

2. Atomic state changes

Operations that must succeed together should execute within the same
database transaction.

3. Explicit transaction states

The system should represent uncertainty and intermediate states instead of
assuming every operation is immediately successful or failed.

4. Idempotent processing

Retries should not automatically create duplicate operations.

5. Failure is expected

Network failures, service failures, duplicate messages, timeouts, and
partial processing are treated as normal engineering scenarios that the
system must handle.

6. Evidence over assumptions

Correctness will be demonstrated through:

unit tests
integration tests
concurrency tests
failure-injection tests
load tests
metrics
traces
reproducible failure scenarios
Build Roadmap
Day	Focus
14	Architecture + Transaction Core
15	PostgreSQL Concurrency + Race Conditions
16	Robust Idempotency
17	Routing + Simulated External Rails
18	RabbitMQ + Transactional Outbox
19	Clearing + Settlement
20	Reconciliation + Recovery
21	Security + Observability + Reliability
22	Chaos + Load Testing

Each stage builds on the previous one.

The objective is not to add technologies for the sake of complexity.

Every component will be introduced because the system has an engineering
problem that requires it.

Day 14 Definition of Done

Day 14 is complete when the following foundation exists:

Go transaction service
PostgreSQL persistence
Transaction API
Request validation
Institution resolution
Exact decimal monetary storage
Atomic liquidity reservation
Database transaction boundary
Basic idempotent replay
Transaction status model
Database constraints
Dockerized local infrastructure
Architecture documentation

The remaining distributed-system, settlement, security, observability,
and chaos-engineering capabilities are intentionally implemented in later
stages.


### One important reason I changed it

Your old file showed **RabbitMQ → Reconciliation → Settlement → Clearing** as though they were already part of the working system. They aren't yet.

This version makes a very clear distinction:

**CURRENT — Day 14**
```text
Client
  ↓
Transaction API
  ↓
Transaction Core
  ↓
PostgreSQL

PLANNED — later days

Routing
Clearing
Settlement
RabbitMQ
Recovery
Reconciliation
Security
Observability
Chaos testing