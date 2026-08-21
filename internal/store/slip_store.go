package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task146-slopestability/internal/model"
)

// --- SlipSurface ---

// CreateSlipSurface inserts a slip-surface row.
func (s *Store) CreateSlipSurface(ctx context.Context, tx *sql.Tx, sf *model.SlipSurface) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	ic := 0
	if sf.IsCritical {
		ic = 1
	}
	_, err := q.ExecContext(ctx, `INSERT INTO slip_surfaces(id,slope_id,type,cx,cz,radius,polyline,is_critical,derived_f,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		sf.ID, sf.SlopeID, string(sf.Type), sf.Cx, sf.Cz, sf.Radius, sf.Polyline, ic, sf.DerivedF, sf.CreatedAt)
	if err != nil {
		return fmt.Errorf("create slip surface: %w", err)
	}
	return nil
}

// GetSlipSurface returns one slip surface.
func (s *Store) GetSlipSurface(ctx context.Context, id string) (*model.SlipSurface, error) {
	return s.getSlipSurface(ctx, s.db, id)
}

// GetSlipSurfaceTx returns one slip surface inside a transaction.
func (s *Store) GetSlipSurfaceTx(ctx context.Context, tx *sql.Tx, id string) (*model.SlipSurface, error) {
	return s.getSlipSurface(ctx, tx, id)
}

func (s *Store) getSlipSurface(ctx context.Context, q DBTX, id string) (*model.SlipSurface, error) {
	var sf model.SlipSurface
	var typ string
	var ic int
	err := q.QueryRowContext(ctx, `SELECT id,slope_id,type,cx,cz,radius,polyline,is_critical,derived_f,created_at FROM slip_surfaces WHERE id=?`, id).
		Scan(&sf.ID, &sf.SlopeID, &typ, &sf.Cx, &sf.Cz, &sf.Radius, &sf.Polyline, &ic, &sf.DerivedF, &sf.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	sf.Type = model.SlipType(typ)
	sf.IsCritical = ic != 0
	return &sf, nil
}

// ListSlipSurfaces returns the slip surfaces of a slope.
func (s *Store) ListSlipSurfaces(ctx context.Context, slopeID string) ([]model.SlipSurface, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,slope_id,type,cx,cz,radius,polyline,is_critical,derived_f,created_at FROM slip_surfaces WHERE slope_id=? ORDER BY created_at`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list slip surfaces: %w", err)
	}
	defer rows.Close()
	var out []model.SlipSurface
	for rows.Next() {
		var sf model.SlipSurface
		var typ string
		var ic int
		if err := rows.Scan(&sf.ID, &sf.SlopeID, &typ, &sf.Cx, &sf.Cz, &sf.Radius, &sf.Polyline, &ic, &sf.DerivedF, &sf.CreatedAt); err != nil {
			return nil, err
		}
		sf.Type = model.SlipType(typ)
		sf.IsCritical = ic != 0
		out = append(out, sf)
	}
	return out, rows.Err()
}

// MarkCriticalSlip sets is_critical=1 and derived_f on a slip surface and
// clears the flag on all other surfaces of the same slope.
func (s *Store) MarkCriticalSlip(ctx context.Context, tx *sql.Tx, slopeID, id string, f float64) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	if _, err := q.ExecContext(ctx, `UPDATE slip_surfaces SET is_critical=0 WHERE slope_id=?`, slopeID); err != nil {
		return fmt.Errorf("clear critical: %w", err)
	}
	if _, err := q.ExecContext(ctx, `UPDATE slip_surfaces SET is_critical=1, derived_f=? WHERE id=?`, f, id); err != nil {
		return fmt.Errorf("mark critical: %w", err)
	}
	return nil
}

// UpdateSlipDerived sets the derived F of a slip surface.
func (s *Store) UpdateSlipDerived(ctx context.Context, tx *sql.Tx, id string, f float64) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	if _, err := q.ExecContext(ctx, `UPDATE slip_surfaces SET derived_f=? WHERE id=?`, f, id); err != nil {
		return fmt.Errorf("update slip derived: %w", err)
	}
	return nil
}
