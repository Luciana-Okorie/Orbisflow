package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type server struct {
	db *pgxpool.Pool
}

func (s *server) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// createTransactionHandler now does real work:
//  1. Requires an Idempotency-Key header. If a transaction with that key
//     already exists, it returns the existing transaction instead of
//     creating a new one (idempotent replay) - full duplicate-request
//     protection (dedup on concurrent identical retries) is still Day 16,
//     but the basic replay-safety is in place from Day 14 since inserting
//     without it would be actively wrong.
//  2. Looks up source/destination institutions by code.
//  3. Atomically reserves liquidity with a single conditional UPDATE
//     (available >= amount), which takes a row lock for the duration of
//     the update - this is what prevents two concurrent transactions from
//     both reading "sufficient balance" and over-approving. Proving this
//     under real concurrency (many goroutines hammering the same account)
//     is Day 15's job; the mechanism has to exist now for creation to be
//     correct at all.
//  4. Inserts the transaction row and returns what Postgres actually
//     stored.
func (s *server) createTransactionHandler(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeErr(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Source == "" || req.Destination == "" || req.Amount == "" || req.Currency == "" || req.Rail == "" {
		writeErr(w, http.StatusBadRequest, "source, destination, amount, currency, and rail are required")
		return
	}

	ctx := r.Context()

	// 1. Idempotent replay: if this key was already used, return the
	// existing transaction rather than creating a duplicate.
	if existing, err := s.findByIdempotencyKey(ctx, idempotencyKey); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(existing)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusInternalServerError, "failed to check idempotency key")
		return
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx) // no-op if committed

	sourceID, err := lookupInstitutionID(ctx, tx, req.Source)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "unknown source institution: "+req.Source)
		return
	}
	destID, err := lookupInstitutionID(ctx, tx, req.Destination)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "unknown destination institution: "+req.Destination)
		return
	}

	// 2. Atomically reserve liquidity. The WHERE clause and the row lock
	// implied by UPDATE happen together, so two concurrent requests
	// against the same account can't both succeed off a stale read.
	var remaining string
	err = tx.QueryRow(ctx, `
		UPDATE liquidity_accounts
		SET available = available - $1, updated_at = now()
		WHERE institution_id = $2 AND currency = $3 AND available >= $1
		RETURNING available
	`, req.Amount, sourceID, req.Currency).Scan(&remaining)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusConflict, "insufficient liquidity or no liquidity account for "+req.Source+" in "+req.Currency)
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to reserve liquidity")
		return
	}

	// 3. Insert the transaction row.
	var txn Transaction
	err = tx.QueryRow(ctx, `
		INSERT INTO transactions (idempotency_key, source_id, destination_id, amount, currency, rail, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'CREATED')
		RETURNING id, amount, currency, rail, status, created_at
	`, idempotencyKey, sourceID, destID, req.Amount, req.Currency, req.Rail).Scan(
		&txn.ID, &txn.Amount, &txn.Currency, &txn.Rail, &txn.Status, &txn.CreatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create transaction")
		return
	}
	txn.Source = req.Source
	txn.Destination = req.Destination

	if err := tx.Commit(ctx); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(txn)
}

// lookupInstitutionID resolves an institution code to its UUID, inside the
// same DB transaction so it sees a consistent snapshot.
func lookupInstitutionID(ctx context.Context, tx pgx.Tx, code string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM institutions WHERE code = $1`, code).Scan(&id)
	return id, err
}

// findByIdempotencyKey looks up a previously created transaction by its
// idempotency key, joining institutions to recover the human-readable codes.
func (s *server) findByIdempotencyKey(ctx context.Context, key string) (*Transaction, error) {
	var txn Transaction
	var createdAt time.Time
	err := s.db.QueryRow(ctx, `
		SELECT t.id, si.code, di.code, t.amount, t.currency, t.rail, t.status, t.created_at
		FROM transactions t
		JOIN institutions si ON si.id = t.source_id
		JOIN institutions di ON di.id = t.destination_id
		WHERE t.idempotency_key = $1
	`, key).Scan(&txn.ID, &txn.Source, &txn.Destination, &txn.Amount, &txn.Currency, &txn.Rail, &txn.Status, &createdAt)
	if err != nil {
		return nil, err
	}
	txn.CreatedAt = createdAt
	return &txn, nil
}
