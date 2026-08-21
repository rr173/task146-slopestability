package store

import (
	"context"
	"database/sql"
	"fmt"

	"task146-slopestability/internal/model"
)

// AppendEvent writes one event to the append-only event stream inside a tx.
// The event stream is the authoritative replay source for ReconcileAll.
func (s *Store) AppendEvent(ctx context.Context, tx *sql.Tx, e *model.Event) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	res, err := q.ExecContext(ctx, `INSERT INTO events(slope_id,ts,kind,payload) VALUES(?,?,?,?)`,
		e.SlopeID, e.Ts, string(e.Kind), e.Payload)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	id, _ := res.LastInsertId()
	e.ID = id
	return nil
}

// ListEvents returns all events for a slope ordered by (ts, id) so replay
// determinism is preserved across events with the same timestamp.
func (s *Store) ListEvents(ctx context.Context, slopeID string) ([]model.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,slope_id,ts,kind,payload FROM events WHERE slope_id=? ORDER BY ts ASC, id ASC`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	var out []model.Event
	for rows.Next() {
		var e model.Event
		var kind string
		if err := rows.Scan(&e.ID, &e.SlopeID, &e.Ts, &kind, &e.Payload); err != nil {
			return nil, err
		}
		e.Kind = model.EventKind(kind)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListEventsTx is the transaction variant (replay inside ReconcileAll must not
// reach for s.db while a writer tx holds the single connection).
func (s *Store) ListEventsTx(ctx context.Context, tx *sql.Tx, slopeID string) ([]model.Event, error) {
	var q DBTX = tx
	rows, err := q.QueryContext(ctx, `SELECT id,slope_id,ts,kind,payload FROM events WHERE slope_id=? ORDER BY ts ASC, id ASC`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list events tx: %w", err)
	}
	defer rows.Close()
	var out []model.Event
	for rows.Next() {
		var e model.Event
		var kind string
		if err := rows.Scan(&e.ID, &e.SlopeID, &e.Ts, &kind, &e.Payload); err != nil {
			return nil, err
		}
		e.Kind = model.EventKind(kind)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListSlopeIDs returns every slope id that has at least one event (the set
// that ReconcileAll must replay).
func (s *Store) ListSlopeIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT slope_id FROM events`)
	if err != nil {
		return nil, fmt.Errorf("list slope ids: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// AppendReconcileLog records one ReconcileAll run for a slope.
func (s *Store) AppendReconcileLog(ctx context.Context, tx *sql.Tx, slopeID string, runAt int64, recomputed, drifts int, note string) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	if _, err := q.ExecContext(ctx, `INSERT INTO reconcile_log(slope_id,run_at,recomputed,drifts,note) VALUES(?,?,?,?,?)`, slopeID, runAt, recomputed, drifts, note); err != nil {
		return fmt.Errorf("append reconcile log: %w", err)
	}
	return nil
}
