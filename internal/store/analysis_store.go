package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task146-slopestability/internal/model"
)

// --- Analysis ---

// CreateAnalysis inserts an analysis row (slices_json holds the slice table).
func (s *Store) CreateAnalysis(ctx context.Context, tx *sql.Tx, a *model.Analysis, slicesJSON string) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO analyses(id,slope_id,slip_surface_id,method,slice_count,kh,kv,surcharge_q,water_table_el,tension_crack_depth,iterations,final_f,status,slices_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.SlopeID, a.SlipSurfaceID, string(a.Method), a.SliceCount, a.Kh, a.Kv, a.SurchargeQ, a.WaterTableEl, a.TensionCrackDepth, a.Iterations, a.FinalF, string(a.Status), slicesJSON, a.CreatedAt)
	if err != nil {
		return fmt.Errorf("create analysis: %w", err)
	}
	return nil
}

// GetAnalysis returns one analysis, including its slice table JSON.
func (s *Store) GetAnalysis(ctx context.Context, id string) (*model.Analysis, string, error) {
	var a model.Analysis
	var method, status string
	var slicesJSON string
	err := s.db.QueryRowContext(ctx, `SELECT id,slope_id,slip_surface_id,method,slice_count,kh,kv,surcharge_q,water_table_el,tension_crack_depth,iterations,final_f,status,slices_json,created_at FROM analyses WHERE id=?`, id).
		Scan(&a.ID, &a.SlopeID, &a.SlipSurfaceID, &method, &a.SliceCount, &a.Kh, &a.Kv, &a.SurchargeQ, &a.WaterTableEl, &a.TensionCrackDepth, &a.Iterations, &a.FinalF, &status, &slicesJSON, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	a.Method = model.AnalysisMethod(method)
	a.Status = model.AnalysisStatus(status)
	return &a, slicesJSON, nil
}

// ListAnalyses returns the analyses for a slope, newest first.
func (s *Store) ListAnalyses(ctx context.Context, slopeID string) ([]model.Analysis, error) {
	return s.listAnalyses(ctx, s.db, slopeID)
}

// ListAnalysesTx returns analyses using the caller transaction.
func (s *Store) ListAnalysesTx(ctx context.Context, tx *sql.Tx, slopeID string) ([]model.Analysis, error) {
	return s.listAnalyses(ctx, tx, slopeID)
}

func (s *Store) listAnalyses(ctx context.Context, q DBTX, slopeID string) ([]model.Analysis, error) {
	rows, err := q.QueryContext(ctx, `SELECT id,slope_id,slip_surface_id,method,slice_count,kh,kv,surcharge_q,water_table_el,tension_crack_depth,iterations,final_f,status,slices_json,created_at FROM analyses WHERE slope_id=? ORDER BY created_at DESC`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list analyses: %w", err)
	}
	defer rows.Close()
	var out []model.Analysis
	for rows.Next() {
		var a model.Analysis
		var method, status, slicesJSON string
		if err := rows.Scan(&a.ID, &a.SlopeID, &a.SlipSurfaceID, &method, &a.SliceCount, &a.Kh, &a.Kv, &a.SurchargeQ, &a.WaterTableEl, &a.TensionCrackDepth, &a.Iterations, &a.FinalF, &status, &slicesJSON, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Method = model.AnalysisMethod(method)
		a.Status = model.AnalysisStatus(status)
		out = append(out, a)
	}
	return out, rows.Err()
}

// --- CriticalSearch ---

// SaveCriticalSearch persists a search summary JSON for a slope.
func (s *Store) SaveCriticalSearch(ctx context.Context, tx *sql.Tx, id, slopeID, summaryJSON string, createdAt int64) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO critical_searches(id,slope_id,summary_json,created_at) VALUES(?,?,?,?)`, id, slopeID, summaryJSON, createdAt)
	if err != nil {
		return fmt.Errorf("save critical search: %w", err)
	}
	return nil
}

// GetCriticalSearch returns one search summary JSON.
func (s *Store) GetCriticalSearch(ctx context.Context, id string) (string, error) {
	var summaryJSON string
	err := s.db.QueryRowContext(ctx, `SELECT summary_json FROM critical_searches WHERE id=?`, id).Scan(&summaryJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return summaryJSON, nil
}

// ListCriticalSearches returns the search history for a slope.
func (s *Store) ListCriticalSearches(ctx context.Context, slopeID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, summary_json FROM critical_searches WHERE slope_id=? ORDER BY created_at DESC`, slopeID)
	if err != nil {
		return nil, fmt.Errorf("list critical searches: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, sj string
		if err := rows.Scan(&id, &sj); err != nil {
			return nil, err
		}
		out[id] = sj
	}
	return out, rows.Err()
}
