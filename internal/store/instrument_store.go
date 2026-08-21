package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task146-slopestability/internal/model"
)

// --- Reinforcement ---

// CreateReinforcement inserts a reinforcement row.
func (s *Store) CreateReinforcement(ctx context.Context, tx *sql.Tx, r *model.Reinforcement) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO reinforcements(id,slope_id,type,tensile_kni,embed_top_el,embed_bot_el,capacity_kn,angle_deg,depth_el,computed_eff,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.SlopeID, string(r.Type), r.TensileKNI, r.EmbedTopEl, r.EmbedBotEl, r.CapacityKN, r.AngleDeg, r.DepthEl, r.ComputedEff, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("create reinforcement: %w", err)
	}
	return nil
}

// ListReinforcements returns the reinforcements for a slope.
func (s *Store) ListReinforcements(ctx context.Context, slopeID string) ([]model.Reinforcement, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,slope_id,type,tensile_kni,embed_top_el,embed_bot_el,capacity_kn,angle_deg,depth_el,computed_eff,created_at FROM reinforcements WHERE slope_id=? ORDER BY created_at`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list reinforcements: %w", err)
	}
	defer rows.Close()
	var out []model.Reinforcement
	for rows.Next() {
		var r model.Reinforcement
		var typ string
		if err := rows.Scan(&r.ID, &r.SlopeID, &typ, &r.TensileKNI, &r.EmbedTopEl, &r.EmbedBotEl, &r.CapacityKN, &r.AngleDeg, &r.DepthEl, &r.ComputedEff, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Type = model.ReinforcementType(typ)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListReinforcementsTx returns reinforcements inside a transaction.
func (s *Store) ListReinforcementsTx(ctx context.Context, tx *sql.Tx, slopeID string) ([]model.Reinforcement, error) {
	var q DBTX = tx
	rows, err := q.QueryContext(ctx, `SELECT id,slope_id,type,tensile_kni,embed_top_el,embed_bot_el,capacity_kn,angle_deg,depth_el,computed_eff,created_at FROM reinforcements WHERE slope_id=? ORDER BY created_at`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list reinforcements tx: %w", err)
	}
	defer rows.Close()
	var out []model.Reinforcement
	for rows.Next() {
		var r model.Reinforcement
		var typ string
		if err := rows.Scan(&r.ID, &r.SlopeID, &typ, &r.TensileKNI, &r.EmbedTopEl, &r.EmbedBotEl, &r.CapacityKN, &r.AngleDeg, &r.DepthEl, &r.ComputedEff, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Type = model.ReinforcementType(typ)
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteReinforcement removes a reinforcement row.
func (s *Store) DeleteReinforcement(ctx context.Context, tx *sql.Tx, id string) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM reinforcements WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete reinforcement: %w", err)
	}
	return nil
}

// --- Instrument ---

// CreateInstrument inserts an instrument row.
func (s *Store) CreateInstrument(ctx context.Context, tx *sql.Tx, ins *model.Instrument) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO instruments(id,slope_id,type,x,install_el,range_max,created_at) VALUES(?,?,?,?,?,?,?)`,
		ins.ID, ins.SlopeID, string(ins.Type), ins.X, ins.InstallEl, ins.RangeMax, ins.CreatedAt)
	if err != nil {
		return fmt.Errorf("create instrument: %w", err)
	}
	return nil
}

// ListInstruments returns the instruments for a slope.
func (s *Store) ListInstruments(ctx context.Context, slopeID string) ([]model.Instrument, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,slope_id,type,x,install_el,range_max,created_at FROM instruments WHERE slope_id=? ORDER BY created_at`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list instruments: %w", err)
	}
	defer rows.Close()
	var out []model.Instrument
	for rows.Next() {
		var ins model.Instrument
		var typ string
		if err := rows.Scan(&ins.ID, &ins.SlopeID, &typ, &ins.X, &ins.InstallEl, &ins.RangeMax, &ins.CreatedAt); err != nil {
			return nil, err
		}
		ins.Type = model.InstrumentType(typ)
		out = append(out, ins)
	}
	return out, rows.Err()
}

// ListInstrumentsTx returns instruments inside a transaction.
func (s *Store) ListInstrumentsTx(ctx context.Context, tx *sql.Tx, slopeID string) ([]model.Instrument, error) {
	var q DBTX = tx
	rows, err := q.QueryContext(ctx, `SELECT id,slope_id,type,x,install_el,range_max,created_at FROM instruments WHERE slope_id=? ORDER BY created_at`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list instruments tx: %w", err)
	}
	defer rows.Close()
	var out []model.Instrument
	for rows.Next() {
		var ins model.Instrument
		var typ string
		if err := rows.Scan(&ins.ID, &ins.SlopeID, &typ, &ins.X, &ins.InstallEl, &ins.RangeMax, &ins.CreatedAt); err != nil {
			return nil, err
		}
		ins.Type = model.InstrumentType(typ)
		out = append(out, ins)
	}
	return out, rows.Err()
}

// GetInstrument returns one instrument.
func (s *Store) GetInstrument(ctx context.Context, id string) (*model.Instrument, error) {
	return s.getInstrument(ctx, s.db, id)
}

// GetInstrumentTx returns one instrument using the caller transaction.
func (s *Store) GetInstrumentTx(ctx context.Context, tx *sql.Tx, id string) (*model.Instrument, error) {
	return s.getInstrument(ctx, tx, id)
}

func (s *Store) getInstrument(ctx context.Context, q DBTX, id string) (*model.Instrument, error) {
	var ins model.Instrument
	var typ string
	err := q.QueryRowContext(ctx, `SELECT id,slope_id,type,x,install_el,range_max,created_at FROM instruments WHERE id=?`, id).
		Scan(&ins.ID, &ins.SlopeID, &typ, &ins.X, &ins.InstallEl, &ins.RangeMax, &ins.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	ins.Type = model.InstrumentType(typ)
	return &ins, nil
}

// --- Reading ---

// CreateReading inserts a reading row.
func (s *Store) CreateReading(ctx context.Context, tx *sql.Tx, rd *model.Reading) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO readings(id,instrument_id,slope_id,ts,value,session_id,source) VALUES(?,?,?,?,?,?,?)`,
		rd.ID, rd.InstrumentID, rd.SlopeID, rd.Ts, rd.Value, rd.SessionID, string(rd.Source))
	if err != nil {
		return fmt.Errorf("create reading: %w", err)
	}
	return nil
}

// ListReadings returns readings for a slope, optionally filtered by instrument.
func (s *Store) ListReadings(ctx context.Context, slopeID, instrumentID string) ([]model.Reading, error) {
	q := `SELECT id,instrument_id,slope_id,ts,value,session_id,source FROM readings WHERE slope_id=?`
	args := []any{slopeID}
	if instrumentID != "" {
		q += ` AND instrument_id=?`
		args = append(args, instrumentID)
	}
	q += ` ORDER BY ts`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list readings: %w", err)
	}
	defer rows.Close()
	var out []model.Reading
	for rows.Next() {
		var rd model.Reading
		var src string
		if err := rows.Scan(&rd.ID, &rd.InstrumentID, &rd.SlopeID, &rd.Ts, &rd.Value, &rd.SessionID, &src); err != nil {
			return nil, err
		}
		rd.Source = model.ReadingSource(src)
		out = append(out, rd)
	}
	return out, rows.Err()
}

// LatestPiezometerReading returns the most recent piezometer reading for a
// slope (the live water-table source used by recompute). Returns ErrNotFound
// when there is none.
func (s *Store) LatestPiezometerReading(ctx context.Context, slopeID string) (*model.Reading, error) {
	var rd model.Reading
	var src string
	err := s.db.QueryRowContext(ctx, `SELECT r.id,r.instrument_id,r.slope_id,r.ts,r.value,r.session_id,r.source
		FROM readings r JOIN instruments i ON i.id=r.instrument_id
		WHERE r.slope_id=? AND i.type='piezometer'
		ORDER BY r.ts DESC LIMIT 1`, slopeID).
		Scan(&rd.ID, &rd.InstrumentID, &rd.SlopeID, &rd.Ts, &rd.Value, &rd.SessionID, &src)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rd.Source = model.ReadingSource(src)
	return &rd, nil
}

// LatestPiezometerReadingTx is the transaction variant.
func (s *Store) LatestPiezometerReadingTx(ctx context.Context, tx *sql.Tx, slopeID string) (*model.Reading, error) {
	var q DBTX = tx
	var rd model.Reading
	var src string
	err := q.QueryRowContext(ctx, `SELECT r.id,r.instrument_id,r.slope_id,r.ts,r.value,r.session_id,r.source
		FROM readings r JOIN instruments i ON i.id=r.instrument_id
		WHERE r.slope_id=? AND i.type='piezometer'
		ORDER BY r.ts DESC LIMIT 1`, slopeID).
		Scan(&rd.ID, &rd.InstrumentID, &rd.SlopeID, &rd.Ts, &rd.Value, &rd.SessionID, &src)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rd.Source = model.ReadingSource(src)
	return &rd, nil
}

// LatestInclinometerReadingTx returns the most recent inclinometer reading.
func (s *Store) LatestInclinometerReadingTx(ctx context.Context, tx *sql.Tx, slopeID string) (*model.Reading, error) {
	var q DBTX = tx
	var rd model.Reading
	var src string
	err := q.QueryRowContext(ctx, `SELECT r.id,r.instrument_id,r.slope_id,r.ts,r.value,r.session_id,r.source
		FROM readings r JOIN instruments i ON i.id=r.instrument_id
		WHERE r.slope_id=? AND i.type='inclinometer'
		ORDER BY r.ts DESC LIMIT 1`, slopeID).
		Scan(&rd.ID, &rd.InstrumentID, &rd.SlopeID, &rd.Ts, &rd.Value, &rd.SessionID, &src)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rd.Source = model.ReadingSource(src)
	return &rd, nil
}

// ListPiezometerReadingsTx returns all piezometer readings for a slope, ordered
// by timestamp ascending. Used by the surge comparison inside a transaction.
func (s *Store) ListPiezometerReadingsTx(ctx context.Context, tx *sql.Tx, slopeID string) ([]model.Reading, error) {
	var q DBTX = tx
	rows, err := q.QueryContext(ctx, `SELECT r.id,r.instrument_id,r.slope_id,r.ts,r.value,r.session_id,r.source
		FROM readings r JOIN instruments i ON i.id=r.instrument_id
		WHERE r.slope_id=? AND i.type='piezometer'
		ORDER BY r.ts ASC`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list piezometer readings tx: %w", err)
	}
	defer rows.Close()
	var out []model.Reading
	for rows.Next() {
		var rd model.Reading
		var src string
		if err := rows.Scan(&rd.ID, &rd.InstrumentID, &rd.SlopeID, &rd.Ts, &rd.Value, &rd.SessionID, &src); err != nil {
			return nil, err
		}
		rd.Source = model.ReadingSource(src)
		out = append(out, rd)
	}
	return out, rows.Err()
}
