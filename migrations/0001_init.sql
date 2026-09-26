CREATE TYPE transaction_status AS ENUM (
    'CREATED',
    'VALIDATING',
    'CLEARED',
    'SETTLEMENT_PENDING',
    'PROCESSING',
    'SUCCESS',
    'FAILED',
    'UNKNOWN'
);

CREATE TABLE institutions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE liquidity_accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id  UUID NOT NULL REFERENCES institutions(id),
    currency        TEXT NOT NULL,
    available       NUMERIC(20, 2) NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (institution_id, currency)
);

CREATE TABLE transactions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key  TEXT NOT NULL UNIQUE,
    source_id        UUID NOT NULL REFERENCES institutions(id),
    destination_id   UUID NOT NULL REFERENCES institutions(id),
    amount           NUMERIC(20, 2) NOT NULL CHECK (amount > 0),
    currency         TEXT NOT NULL,
    rail             TEXT NOT NULL,
    status           transaction_status NOT NULL DEFAULT 'CREATED',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_source ON transactions(source_id);
CREATE INDEX idx_transactions_destination ON transactions(destination_id);

-- Outbox table (populated in the same DB transaction as a transactions
-- write; a separate worker publishes these to RabbitMQ). Not consumed yet
-- as of Day 14 - wired up on Day 18.
CREATE TABLE outbox_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id UUID NOT NULL,
    event_type   TEXT NOT NULL,
    payload      JSONB NOT NULL,
    published    BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
