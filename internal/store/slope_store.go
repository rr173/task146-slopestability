package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task146-slopestability/internal/model"
)

// --- Slope ---

// CreateSlope inserts a slope row.
func (s *Store) CreateSlope(ctx context.Context, tx *sql.Tx, sl *model.Slope) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO slopes(id,name,crest_el,toe_el,height,slope_angle,water_table_el,surcharge_x,surcharge_q,tension_crack_el,status,current_f,alert_level,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sl.ID, sl.Name, sl.CrestEl, sl.ToeEl, sl.Height, sl.SlopeAngle, sl.WaterTableEl, sl.SurchargeX, sl.SurchargeQ, sl.TensionCrackEl, string(sl.Status), sl.CurrentF, string(sl.AlertLevel), sl.CreatedAt)
	if err != nil {
		return fmt.Errorf("create slope: %w", err)
	}
	return nil
}

// GetSlope returns one slope.
func (s *Store) GetSlope(ctx context.Context, id string) (*model.Slope, error) {
	return s.getSlope(ctx, s.db, id)
}

// GetSlopeTx returns one slope inside a transaction.
func (s *Store) GetSlopeTx(ctx context.Context, tx *sql.Tx, id string) (*model.Slope, error) {
	return s.getSlope(ctx, tx, id)
}

func (s *Store) getSlope(ctx context.Context, q DBTX, id string) (*model.Slope, error) {
	var sl model.Slope
	var status, alert string
	err := q.QueryRowContext(ctx, `SELECT id,name,crest_el,toe_el,height,slope_angle,water_table_el,surcharge_x,surcharge_q,tension_crack_el,status,current_f,alert_level,created_at FROM slopes WHERE id=?`, id).
		Scan(&sl.ID, &sl.Name, &sl.CrestEl, &sl.ToeEl, &sl.Height, &sl.SlopeAngle, &sl.WaterTableEl, &sl.SurchargeX, &sl.SurchargeQ, &sl.TensionCrackEl, &status, &sl.CurrentF, &alert, &sl.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	sl.Status = model.SlopeStatus(status)
	sl.AlertLevel = model.AlertLevel(alert)
	return &sl, nil
}

// ListSlopes returns all slopes ordered by creation time.
func (s *Store) ListSlopes(ctx context.Context) ([]model.Slope, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,crest_el,toe_el,height,slope_angle,water_table_el,surcharge_x,surcharge_q,tension_crack_el,status,current_f,alert_level,created_at FROM slopes ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list slopes: %w", err)
	}
	defer rows.Close()
	var out []model.Slope
	for rows.Next() {
		var sl model.Slope
		var status, alert string
		if err := rows.Scan(&sl.ID, &sl.Name, &sl.CrestEl, &sl.ToeEl, &sl.Height, &sl.SlopeAngle, &sl.WaterTableEl, &sl.SurchargeX, &sl.SurchargeQ, &sl.TensionCrackEl, &status, &sl.CurrentF, &alert, &sl.CreatedAt); err != nil {
			return nil, err
		}
		sl.Status = model.SlopeStatus(status)
		sl.AlertLevel = model.AlertLevel(alert)
		out = append(out, sl)
	}
	return out, rows.Err()
}

// UpdateSlopeGeometry updates the editable geometry and water table fields.
func (s *Store) UpdateSlopeGeometry(ctx context.Context, tx *sql.Tx, sl *model.Slope) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE slopes SET name=?,crest_el=?,toe_el=?,height=?,slope_angle=?,water_table_el=?,surcharge_x=?,surcharge_q=?,tension_crack_el=? WHERE id=?`,
		sl.Name, sl.CrestEl, sl.ToeEl, sl.Height, sl.SlopeAngle, sl.WaterTableEl, sl.SurchargeX, sl.SurchargeQ, sl.TensionCrackEl, sl.ID)
	if err != nil {
		return fmt.Errorf("update slope geometry: %w", err)
	}
	return nil
}

// UpdateSlopeStatus sets the lifecycle status.
func (s *Store) UpdateSlopeStatus(ctx context.Context, tx *sql.Tx, id string, status model.SlopeStatus) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE slopes SET status=? WHERE id=?`, string(status), id)
	if err != nil {
		return fmt.Errorf("update slope status: %w", err)
	}
	return nil
}

// UpdateSlopeDerived sets the recomputed current_f and alert_level (derived).
func (s *Store) UpdateSlopeDerived(ctx context.Context, tx *sql.Tx, id string, currentF float64, alert model.AlertLevel) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE slopes SET current_f=?, alert_level=? WHERE id=?`, currentF, string(alert), id)
	if err != nil {
		return fmt.Errorf("update slope derived: %w", err)
	}
	return nil
}

// DeleteSlope removes a slope (caller ensures it is not monitoring).
func (s *Store) DeleteSlope(ctx context.Context, tx *sql.Tx, id string) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM slopes WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete slope: %w", err)
	}
	return nil
}

// --- SoilLayer ---

// CreateLayer inserts a soil-layer row.
func (s *Store) CreateLayer(ctx context.Context, tx *sql.Tx, l *model.SoilLayer) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO soil_layers(id,slope_id,name,c,phi,gamma,top_el,bot_el,ord) VALUES(?,?,?,?,?,?,?,?,?)`,
		l.ID, l.SlopeID, l.Name, l.C, l.Phi, l.Gamma, l.TopEl, l.BotEl, l.Order)
	if err != nil {
		return fmt.Errorf("create layer: %w", err)
	}
	return nil
}

// ListLayers returns the layers of a slope ordered top-down.
func (s *Store) ListLayers(ctx context.Context, slopeID string) ([]model.SoilLayer, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,slope_id,name,c,phi,gamma,top_el,bot_el,ord FROM soil_layers WHERE slope_id=? ORDER BY ord`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list layers: %w", err)
	}
	defer rows.Close()
	var out []model.SoilLayer
	for rows.Next() {
		var l model.SoilLayer
		if err := rows.Scan(&l.ID, &l.SlopeID, &l.Name, &l.C, &l.Phi, &l.Gamma, &l.TopEl, &l.BotEl, &l.Order); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ListLayersTx returns the layers inside a transaction.
func (s *Store) ListLayersTx(ctx context.Context, tx *sql.Tx, slopeID string) ([]model.SoilLayer, error) {
	var q DBTX = tx
	rows, err := q.QueryContext(ctx, `SELECT id,slope_id,name,c,phi,gamma,top_el,bot_el,ord FROM soil_layers WHERE slope_id=? ORDER BY ord`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list layers tx: %w", err)
	}
	defer rows.Close()
	var out []model.SoilLayer
	for rows.Next() {
		var l model.SoilLayer
		if err := rows.Scan(&l.ID, &l.SlopeID, &l.Name, &l.C, &l.Phi, &l.Gamma, &l.TopEl, &l.BotEl, &l.Order); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetLayer returns one soil layer by id.
func (s *Store) GetLayer(ctx context.Context, id string) (*model.SoilLayer, error) {
	var l model.SoilLayer
	err := s.db.QueryRowContext(ctx, `SELECT id,slope_id,name,c,phi,gamma,top_el,bot_el,ord FROM soil_layers WHERE id=?`, id).
		Scan(&l.ID, &l.SlopeID, &l.Name, &l.C, &l.Phi, &l.Gamma, &l.TopEl, &l.BotEl, &l.Order)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &l, nil
}

// UpdateLayer updates a layer's parameters.
func (s *Store) UpdateLayer(ctx context.Context, tx *sql.Tx, l *model.SoilLayer) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE soil_layers SET name=?,c=?,phi=?,gamma=?,top_el=?,bot_el=?,ord=? WHERE id=?`,
		l.Name, l.C, l.Phi, l.Gamma, l.TopEl, l.BotEl, l.Order, l.ID)
	if err != nil {
		return fmt.Errorf("update layer: %w", err)
	}
	return nil
}

// DeleteLayer removes a layer.
func (s *Store) DeleteLayer(ctx context.Context, tx *sql.Tx, id string) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM soil_layers WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete layer: %w", err)
	}
	return nil
}
