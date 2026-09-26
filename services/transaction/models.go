package main

import "time"

type TransactionStatus string

const (
	StatusCreated           TransactionStatus = "CREATED"
	StatusValidating        TransactionStatus = "VALIDATING"
	StatusCleared           TransactionStatus = "CLEARED"
	StatusSettlementPending TransactionStatus = "SETTLEMENT_PENDING"
	StatusProcessing        TransactionStatus = "PROCESSING"
	StatusSuccess           TransactionStatus = "SUCCESS"
	StatusFailed            TransactionStatus = "FAILED"
	StatusUnknown           TransactionStatus = "UNKNOWN"
)

// CreateTransactionRequest mirrors the POST /v1/transactions body from the
// project spec. The Idempotency-Key is sent as a header, not a body field,
// per the original spec (Idempotency-Key: 7d8f...).
type CreateTransactionRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	Rail        string `json:"rail"`
}

type Transaction struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Destination string            `json:"destination"`
	Amount      string            `json:"amount"`
	Currency    string            `json:"currency"`
	Rail        string            `json:"rail"`
	Status      TransactionStatus `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
}
