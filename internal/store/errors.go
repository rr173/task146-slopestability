package store

import "errors"

// Domain errors mapped to HTTP statuses by the httpapi layer. Each is a
// sentinel so the service can wrap it with %w and the handler can errors.Is it.
var (
	ErrNotFound         = errors.New("store: not found")
	ErrConflict         = errors.New("store: conflict")
	ErrStateConflict    = errors.New("store: illegal state transition")
	ErrInvariant        = errors.New("store: invariant violation")
	ErrGeometry         = errors.New("store: illegal slip-surface geometry")
	ErrNotConverged     = errors.New("store: analysis did not converge")
	ErrAlertBlocked     = errors.New("store: red alert blocks closure")
	ErrNotReconciled    = errors.New("store: slope not reconciled")
	ErrReadingStale     = errors.New("store: reading is stale relative to latest")
)
