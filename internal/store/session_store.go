package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task146-slopestability/internal/model"
)

// --- Session ---

// CreateSession inserts a monitoring session row.
func (s *Store) CreateSession(ctx context.Context, tx *sql.Tx, ses *model.Session) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	rec := 0
	if ses.Reconciled {
		rec = 1
	}
	_, err := q.ExecContext(ctx, `INSERT INTO sessions(id,slope_id,created_at,note,recomputed_f,alert_level,reconciled,prev_alert) VALUES(?,?,?,?,?,?,?,?)`,
		ses.ID, ses.SlopeID, ses.CreatedAt, ses.Note, ses.RecomputedF, string(ses.AlertLevel), rec, string(ses.PrevAlert))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// GetSession returns one session.
func (s *Store) GetSession(ctx context.Context, id string) (*model.Session, error) {
	return s.getSession(ctx, s.db, id)
}

// GetSessionTx returns one session using the caller transaction.
func (s *Store) GetSessionTx(ctx context.Context, tx *sql.Tx, id string) (*model.Session, error) {
	return s.getSession(ctx, tx, id)
}

func (s *Store) getSession(ctx context.Context, q DBTX, id string) (*model.Session, error) {
	var ses model.Session
	var alert, prev string
	var rec int
	err := q.QueryRowContext(ctx, `SELECT id,slope_id,created_at,note,recomputed_f,alert_level,reconciled,prev_alert FROM sessions WHERE id=?`, id).
		Scan(&ses.ID, &ses.SlopeID, &ses.CreatedAt, &ses.Note, &ses.RecomputedF, &alert, &rec, &prev)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	ses.AlertLevel = model.AlertLevel(alert)
	ses.PrevAlert = model.AlertLevel(prev)
	ses.Reconciled = rec != 0
	return &ses, nil
}

// ListSessions returns the sessions for a slope, newest first.
func (s *Store) ListSessions(ctx context.Context, slopeID string) ([]model.Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,slope_id,created_at,note,recomputed_f,alert_level,reconciled,prev_alert FROM sessions WHERE slope_id=? ORDER BY created_at DESC`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var out []model.Session
	for rows.Next() {
		var ses model.Session
		var alert, prev string
		var rec int
		if err := rows.Scan(&ses.ID, &ses.SlopeID, &ses.CreatedAt, &ses.Note, &ses.RecomputedF, &alert, &rec, &prev); err != nil {
			return nil, err
		}
		ses.AlertLevel = model.AlertLevel(alert)
		ses.PrevAlert = model.AlertLevel(prev)
		ses.Reconciled = rec != 0
		out = append(out, ses)
	}
	return out, rows.Err()
}

// UpdateSessionResult sets the recomputed F and alert level on a session and
// marks it reconciled.
func (s *Store) UpdateSessionResult(ctx context.Context, tx *sql.Tx, id string, f float64, alert model.AlertLevel, reconciled bool) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	rec := 0
	if reconciled {
		rec = 1
	}
	if _, err := q.ExecContext(ctx, `UPDATE sessions SET recomputed_f=?, alert_level=?, reconciled=? WHERE id=?`, f, string(alert), rec, id); err != nil {
		return fmt.Errorf("update session result: %w", err)
	}
	return nil
}

// --- Compliance ---

// CreateCompliance inserts a compliance verdict row.
func (s *Store) CreateCompliance(ctx context.Context, tx *sql.Tx, c *model.Compliance) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO compliances(id,slope_id,session_id,scenario,required_f,actual_f,verdict,detail) VALUES(?,?,?,?,?,?,?,?)`,
		c.ID, c.SlopeID, c.SessionID, string(c.Scenario), c.RequiredF, c.ActualF, c.Verdict, c.Detail)
	if err != nil {
		return fmt.Errorf("create compliance: %w", err)
	}
	return nil
}

// ListCompliance returns the compliance verdicts for a slope.
func (s *Store) ListCompliance(ctx context.Context, slopeID string) ([]model.Compliance, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,slope_id,session_id,scenario,required_f,actual_f,verdict,detail FROM compliances WHERE slope_id=? ORDER BY id`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list compliance: %w", err)
	}
	defer rows.Close()
	var out []model.Compliance
	for rows.Next() {
		var c model.Compliance
		var scen string
		if err := rows.Scan(&c.ID, &c.SlopeID, &c.SessionID, &scen, &c.RequiredF, &c.ActualF, &c.Verdict, &c.Detail); err != nil {
			return nil, err
		}
		c.Scenario = model.ComplianceScenario(scen)
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteComplianceForSlope removes existing compliance rows for a slope (used
// before re-writing on recompute so the latest verdict is authoritative).
func (s *Store) DeleteComplianceForSlope(ctx context.Context, tx *sql.Tx, slopeID string) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM compliances WHERE slope_id=?`, slopeID); err != nil {
		return fmt.Errorf("delete compliance: %w", err)
	}
	return nil
}
