package store

import (
	"context"
	"database/sql"
	"fmt"
)

// ReconcileSummary is the result of a single slope's replay.
type ReconcileSummary struct {
	SlopeID    string
	Recomputed int  // number of derived fields written back
	Drifts     int  // number of figures that disagreed with the stored value
	StoredF    float64
	ComputedF  float64
}

// ReconcileSlope replays a slope's events and is the persistence hook the
// service's ReconcileAll calls. It only persists the reconcile_log row; the
// actual recompute (re-deriving F from geometry/layers/readings) is done by
// the service layer, which owns the geotech solver. The store stays free of
// geotech imports.
func (s *Store) ReconcileSlope(ctx context.Context, slopeID string, runAt int64, recomputed, drifts int, note string) error {
	return s.InTx(ctx, func(tx *sql.Tx) error {
		return s.AppendReconcileLog(ctx, tx, slopeID, runAt, recomputed, drifts, note)
	})
}

// SchemaVersion returns the schema_meta value for "version".
func (s *Store) SchemaVersion(ctx context.Context) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM schema_meta WHERE key='version'`).Scan(&v)
	if err != nil {
		return "", fmt.Errorf("schema version: %w", err)
	}
	return v, nil
}

// SetSchemaVersion records the schema version stamp.
func (s *Store) SetSchemaVersion(ctx context.Context, v string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO schema_meta(key,value) VALUES('version',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, v)
	if err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}

// LatestReconcileAt returns the most recent reconcile_log run_at for a slope
// (or 0 if none).
func (s *Store) LatestReconcileAt(ctx context.Context, slopeID string) (int64, error) {
	var runAt sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(run_at) FROM reconcile_log WHERE slope_id=?`, slopeID).Scan(&runAt)
	if err != nil {
		return 0, fmt.Errorf("latest reconcile at: %w", err)
	}
	if !runAt.Valid {
		return 0, nil
	}
	return runAt.Int64, nil
}
