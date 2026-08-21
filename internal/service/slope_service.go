package service

import (
	"context"
	"database/sql"
	"fmt"

	"task146-slopestability/internal/model"
	"task146-slopestability/internal/store"
)

// --- Slope lifecycle ---

// CreateSlopeInput is the body for creating a slope.
type CreateSlopeInput struct {
	Name           string  `json:"name"`
	CrestEl        float64 `json:"crest_el"`
	ToeEl          float64 `json:"toe_el"`
	SlopeAngle     float64 `json:"slope_angle"`
	WaterTableEl   float64 `json:"water_table_el"`
	SurchargeX     float64 `json:"surcharge_x"`
	SurchargeQ     float64 `json:"surcharge_q"`
	TensionCrackEl float64 `json:"tension_crack_el"`
}

// CreateSlope validates the geometry and persists a new slope.
func (s *Service) CreateSlope(ctx context.Context, in CreateSlopeInput) (*model.Slope, error) {
	if err := validateSlopeGeometry(in.CrestEl, in.ToeEl, in.SlopeAngle); err != nil {
		return nil, err
	}
	sl := &model.Slope{
		ID: nextID("slp"), Name: in.Name, CrestEl: in.CrestEl, ToeEl: in.ToeEl,
		Height: in.CrestEl - in.ToeEl, SlopeAngle: in.SlopeAngle,
		WaterTableEl: in.WaterTableEl, SurchargeX: in.SurchargeX, SurchargeQ: in.SurchargeQ,
		TensionCrackEl: in.TensionCrackEl, Status: model.StatusInvestigating,
		AlertLevel: model.AlertGreen, CreatedAt: s.now(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		if err := s.store.CreateSlope(ctx, tx, sl); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: sl.ID, Ts: sl.CreatedAt, Kind: model.EvSlopeCreate,
			Payload: marshalPayload(sl),
		})
	})
	if err != nil {
		return nil, err
	}
	return sl, nil
}

// GetSlope returns a slope with its derived current F.
func (s *Service) GetSlope(ctx context.Context, id string) (*model.Slope, error) {
	return s.store.GetSlope(ctx, id)
}

// ListSlopes returns all slopes.
func (s *Service) ListSlopes(ctx context.Context) ([]model.Slope, error) {
	return s.store.ListSlopes(ctx)
}

// UpdateSlopeGeometryInput patches the editable geometry.
type UpdateSlopeGeometryInput struct {
	Name           *string  `json:"name"`
	CrestEl        *float64 `json:"crest_el"`
	ToeEl          *float64 `json:"toe_el"`
	SlopeAngle     *float64 `json:"slope_angle"`
	WaterTableEl   *float64 `json:"water_table_el"`
	SurchargeX     *float64 `json:"surcharge_x"`
	SurchargeQ     *float64 `json:"surcharge_q"`
	TensionCrackEl *float64 `json:"tension_crack_el"`
}

// UpdateSlopeGeometry patches editable geometry; geometry edits are refused on
// a closed slope.
func (s *Service) UpdateSlopeGeometry(ctx context.Context, id string, in UpdateSlopeGeometryInput) (*model.Slope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out *model.Slope
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		sl, err := s.store.GetSlopeTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if sl.Status == model.StatusClosed {
			return store.ErrStateConflict
		}
		if in.Name != nil {
			sl.Name = *in.Name
		}
		if in.CrestEl != nil {
			sl.CrestEl = *in.CrestEl
		}
		if in.ToeEl != nil {
			sl.ToeEl = *in.ToeEl
		}
		if in.SlopeAngle != nil {
			sl.SlopeAngle = *in.SlopeAngle
		}
		if in.WaterTableEl != nil {
			sl.WaterTableEl = *in.WaterTableEl
		}
		if in.SurchargeX != nil {
			sl.SurchargeX = *in.SurchargeX
		}
		if in.SurchargeQ != nil {
			sl.SurchargeQ = *in.SurchargeQ
		}
		if in.TensionCrackEl != nil {
			sl.TensionCrackEl = *in.TensionCrackEl
		}
		if err := validateSlopeGeometry(sl.CrestEl, sl.ToeEl, sl.SlopeAngle); err != nil {
			return err
		}
		sl.Height = sl.CrestEl - sl.ToeEl
		if err := s.store.UpdateSlopeGeometry(ctx, tx, sl); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: sl.ID, Ts: s.now(), Kind: model.EvSlopeUpdate, Payload: marshalPayload(in),
		})
	})
	if err != nil {
		return nil, err
	}
	out, err = s.store.GetSlope(ctx, id)
	return out, err
}

// DeleteSlope removes a slope; refused while monitoring.
func (s *Service) DeleteSlope(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.InTx(ctx, func(tx *sql.Tx) error {
		sl, err := s.store.GetSlopeTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if sl.Status == model.StatusMonitoring || sl.Status == model.StatusAnalyzing {
			return store.ErrStateConflict
		}
		return s.store.DeleteSlope(ctx, tx, id)
	})
}

// CloseSlope closes monitoring; refused when a red alert is active. This is the
// monotone-alert guard (boundary constraint #5).
func (s *Service) CloseSlope(ctx context.Context, id string) (*model.Slope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out *model.Slope
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		sl, err := s.store.GetSlopeTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if sl.AlertLevel == model.AlertRed {
			return store.ErrAlertBlocked
		}
		if sl.Status != model.StatusMonitoring && sl.Status != model.StatusAnalyzing {
			return store.ErrStateConflict
		}
		sl.Status = model.StatusClosed
		if err := s.store.UpdateSlopeStatus(ctx, tx, id, sl.Status); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: id, Ts: s.now(), Kind: model.EvSlopeStateChange,
			Payload: marshalPayload(map[string]string{"to": string(sl.Status)}),
		})
	})
	if err != nil {
		return nil, err
	}
	out, err = s.store.GetSlope(ctx, id)
	return out, err
}

func validateSlopeGeometry(crest, toe, angle float64) error {
	if crest <= toe {
		return fmt.Errorf("%w: crest must be above toe", store.ErrInvariant)
	}
	if angle <= 0 || angle >= 90 {
		return fmt.Errorf("%w: slope_angle must be in (0,90)", store.ErrInvariant)
	}
	return nil
}
