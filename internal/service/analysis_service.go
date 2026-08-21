package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task146-slopestability/internal/geotech"
	"task146-slopestability/internal/model"
	"task146-slopestability/internal/store"
)

// --- SlipSurface ---

// SlipInput is the body for creating a slip surface.
type SlipInput struct {
	Type     string  `json:"type"`
	Cx       float64 `json:"cx"`
	Cz       float64 `json:"cz"`
	Radius   float64 `json:"radius"`
	Polyline string  `json:"polyline"`
}

// CreateSlipSurface persists a trial slip surface.
func (s *Service) CreateSlipSurface(ctx context.Context, slopeID string, in SlipInput) (*model.SlipSurface, error) {
	if in.Type == "" {
		in.Type = string(model.SlipCircular)
	}
	if in.Type == string(model.SlipCircular) && in.Radius <= 0 {
		return nil, fmt.Errorf("%w: circular slip needs radius > 0", store.ErrInvariant)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sf := &model.SlipSurface{
		ID: nextID("ssf"), SlopeID: slopeID, Type: model.SlipType(in.Type),
		Cx: in.Cx, Cz: in.Cz, Radius: in.Radius, Polyline: in.Polyline,
		CreatedAt: s.now(),
	}
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		if _, err := s.store.GetSlopeTx(ctx, tx, slopeID); err != nil {
			return err
		}
		return s.store.CreateSlipSurface(ctx, tx, sf)
	})
	if err != nil {
		return nil, err
	}
	return sf, nil
}

// ListSlipSurfaces returns slip surfaces for a slope.
func (s *Service) ListSlipSurfaces(ctx context.Context, slopeID string) ([]model.SlipSurface, error) {
	if _, err := s.store.GetSlope(ctx, slopeID); err != nil {
		return nil, err
	}
	return s.store.ListSlipSurfaces(ctx, slopeID)
}

// --- Analysis ---

// AnalysisInput is the body for submitting an analysis run.
type AnalysisInput struct {
	SlipSurfaceID     string  `json:"slip_surface_id"`
	Method            string  `json:"method"`
	SliceCount        int     `json:"slice_count"`
	Kh                float64 `json:"kh"`
	Kv                float64 `json:"kv"`
	SurchargeQ        float64 `json:"surcharge_q"`
	WaterTableEl      float64 `json:"water_table_el"`
	TensionCrackDepth float64 `json:"tension_crack_depth"`
}

// AnalysisResult is the response for a submitted analysis.
type AnalysisResult struct {
	*model.Analysis
	Slices []model.SliceResult `json:"slices"`
	Capped bool                `json:"capped"`
}

// SubmitAnalysis runs the solver for a slip surface and stores the result.
func (s *Service) SubmitAnalysis(ctx context.Context, slopeID string, in AnalysisInput) (*AnalysisResult, error) {
	if err := validateAnalysis(in); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	method := model.AnalysisMethod(in.Method)
	if method == "" {
		method = model.MethodBishop
	}

	var newAnalysisID string
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		sl, err := s.store.GetSlopeTx(ctx, tx, slopeID)
		if err != nil {
			return err
		}
		if sl.Status == model.StatusClosed {
			return store.ErrStateConflict
		}
		layers, err := s.store.ListLayersTx(ctx, tx, slopeID)
		if err != nil {
			return err
		}
		if len(layers) == 0 {
			return fmt.Errorf("%w: slope has no soil layers", store.ErrInvariant)
		}
		sf, err := s.store.GetSlipSurfaceTx(ctx, tx, in.SlipSurfaceID)
		if err != nil {
			return err
		}
		if sf.SlopeID != slopeID {
			return store.ErrNotFound
		}
		if sf.Type != model.SlipCircular {
			return store.ErrGeometry
		}
		reinf, err := s.store.ListReinforcementsTx(ctx, tx, slopeID)
		if err != nil {
			return err
		}
		waterTable := sl.WaterTableEl
		if pr, err := s.store.LatestPiezometerReadingTx(ctx, tx, slopeID); err == nil {
			waterTable = pr.Value
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		} else if waterTable <= 0 {
			waterTable = sl.WaterTableEl
		}
		prof := profileFromSlope(sl)
		prof.SurchargeQ = in.SurchargeQ
		if in.TensionCrackDepth > 0 {
			prof.TensionCrackEl = sl.CrestEl - in.TensionCrackDepth
		}
		gin := geotech.SolveInput{
			Profile: prof, Layers: layers, Cx: sf.Cx, Cz: sf.Cz, R: sf.Radius,
			N: in.SliceCount, WaterTableEl: waterTable, Kh: in.Kh, Kv: in.Kv,
			Reinforcements: reinforcementInputs(reinf, layers, sl),
		}

		res, serr := solveByMethod(method, gin)
		if serr != nil && res.F == 0 && method != model.MethodBishop && method != model.MethodFellenius && method != model.MethodJanbu {
			return serr
		}
		status := model.AnalysisConverged
		finalF := res.F
		if serr != nil {
			status = model.AnalysisFailed
		}
		a := &model.Analysis{
			ID: nextID("anl"), SlopeID: slopeID, SlipSurfaceID: in.SlipSurfaceID,
			Method: method, SliceCount: in.SliceCount, Kh: in.Kh, Kv: in.Kv,
			SurchargeQ: in.SurchargeQ, WaterTableEl: waterTable,
			TensionCrackDepth: in.TensionCrackDepth, Iterations: res.Iterations,
			FinalF: finalF, Status: status, CreatedAt: s.now(),
		}
		newAnalysisID = a.ID
		slicesJSON := marshalPayload(res.Slices)
		if err := s.store.CreateAnalysis(ctx, tx, a, slicesJSON); err != nil {
			return err
		}
		// Store the derived F on the slip surface and bump the slope state.
		if err := s.store.UpdateSlipDerived(ctx, tx, in.SlipSurfaceID, finalF); err != nil {
			return err
		}
		if sl.Status == model.StatusInvestigating || sl.Status == model.StatusDesigned {
			if err := s.store.UpdateSlopeStatus(ctx, tx, slopeID, model.StatusAnalyzing); err != nil {
				return err
			}
		}
		// Update the live current_f (first analysis or no monitoring yet).
		if sl.Status != model.StatusMonitoring {
			alert := alertFromF(finalF)
			if err := s.store.UpdateSlopeDerived(ctx, tx, slopeID, finalF, alert); err != nil {
				return err
			}
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: slopeID, Ts: s.now(), Kind: model.EvAnalysisRun, Payload: marshalPayload(a),
		})
	})
	if err != nil {
		return nil, err
	}
	a, slicesJSON, err := s.store.GetAnalysis(ctx, newAnalysisID)
	if err != nil {
		return nil, err
	}
	var slices []model.SliceResult
	if slicesJSON != "" {
		_ = unmarshalPayload(slicesJSON, &slices)
	}
	return &AnalysisResult{Analysis: a, Slices: slices}, nil
}

// GetAnalysis returns one analysis with its slice table.
func (s *Service) GetAnalysis(ctx context.Context, id string) (*AnalysisResult, error) {
	a, slicesJSON, err := s.store.GetAnalysis(ctx, id)
	if err != nil {
		return nil, err
	}
	var slices []model.SliceResult
	if slicesJSON != "" {
		_ = unmarshalPayload(slicesJSON, &slices)
	}
	return &AnalysisResult{Analysis: a, Slices: slices}, nil
}

// ListAnalyses returns analyses for a slope.
func (s *Service) ListAnalyses(ctx context.Context, slopeID string) ([]model.Analysis, error) {
	if _, err := s.store.GetSlope(ctx, slopeID); err != nil {
		return nil, err
	}
	return s.store.ListAnalyses(ctx, slopeID)
}

// SearchCriticalInput is the body for a critical-surface grid search.
type SearchCriticalInput struct {
	Method string `json:"method"`
	Grid   struct {
		CxMin  float64 `json:"cx_min"`
		CxMax  float64 `json:"cx_max"`
		CzMin  float64 `json:"cz_min"`
		CzMax  float64 `json:"cz_max"`
		RMin   float64 `json:"r_min"`
		RMax   float64 `json:"r_max"`
		CxStep float64 `json:"cx_step"`
		CzStep float64 `json:"cz_step"`
		RStep  float64 `json:"r_step"`
	} `json:"grid"`
}

// SearchCritical runs the grid search, stores the winning surface as the
// critical slip surface, and returns the summary.
func (s *Service) SearchCritical(ctx context.Context, slopeID string, in SearchCriticalInput) (*model.SearchSummary, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	method := model.AnalysisMethod(in.Method)
	if method == "" {
		method = model.MethodBishop
	}
	var summary model.SearchSummary
	var newSlipID string
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		sl, err := s.store.GetSlopeTx(ctx, tx, slopeID)
		if err != nil {
			return err
		}
		if sl.Status == model.StatusClosed {
			return store.ErrStateConflict
		}
		layers, err := s.store.ListLayersTx(ctx, tx, slopeID)
		if err != nil {
			return err
		}
		if len(layers) == 0 {
			return fmt.Errorf("%w: slope has no soil layers", store.ErrInvariant)
		}
		prof := profileFromSlope(sl)
		gin := geotech.SolveInput{Profile: prof, Layers: layers, N: 20, WaterTableEl: sl.WaterTableEl}
		grid := geotech.GridBounds{
			CxMin: in.Grid.CxMin, CxMax: in.Grid.CxMax, CzMin: in.Grid.CzMin, CzMax: in.Grid.CzMax,
			RMin: in.Grid.RMin, RMax: in.Grid.RMax, CxStep: in.Grid.CxStep, CzStep: in.Grid.CzStep, RStep: in.Grid.RStep,
		}
		summary, err = geotech.SearchCritical(gin, grid)
		if err != nil {
			return err
		}
		sf := &model.SlipSurface{
			ID: nextID("ssf"), SlopeID: slopeID, Type: model.SlipCircular,
			Cx: summary.BestCx, Cz: summary.BestCz, Radius: summary.BestR,
			IsCritical: true, DerivedF: summary.MinF, CreatedAt: s.now(),
		}
		newSlipID = sf.ID
		if err := s.store.MarkCriticalSlip(ctx, tx, slopeID, sf.ID, summary.MinF); err != nil {
			// surface doesn't exist yet; create it then mark.
			if err := s.store.CreateSlipSurface(ctx, tx, sf); err != nil {
				return err
			}
			if err := s.store.MarkCriticalSlip(ctx, tx, slopeID, sf.ID, summary.MinF); err != nil {
				return err
			}
		}
		searchID := nextID("srh")
		if err := s.store.SaveCriticalSearch(ctx, tx, searchID, slopeID, marshalPayload(summary), s.now()); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: slopeID, Ts: s.now(), Kind: model.EvSlipCritical, Payload: marshalPayload(summary),
		})
	})
	if err != nil {
		return nil, "", err
	}
	return &summary, newSlipID, nil
}

// GetCriticalSearch returns a stored search summary JSON.
func (s *Service) GetCriticalSearch(ctx context.Context, id string) (string, error) {
	return s.store.GetCriticalSearch(ctx, id)
}

func validateAnalysis(in AnalysisInput) error {
	if in.SlipSurfaceID == "" {
		return fmt.Errorf("%w: slip_surface_id required", store.ErrInvariant)
	}
	if in.SliceCount < 5 || in.SliceCount > 200 {
		return fmt.Errorf("%w: slice_count must be in [5,200]", store.ErrInvariant)
	}
	if in.Kh < 0 || in.Kh > 0.5 {
		return fmt.Errorf("%w: kh must be in [0,0.5]", store.ErrInvariant)
	}
	if in.Kv < 0 || in.Kv > 0.5 {
		return fmt.Errorf("%w: kv must be in [0,0.5]", store.ErrInvariant)
	}
	return nil
}
