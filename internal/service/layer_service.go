package service

import (
	"context"
	"database/sql"
	"fmt"

	"task146-slopestability/internal/model"
	"task146-slopestability/internal/store"
)

// --- SoilLayer ---

// LayerInput is the body for adding/updating a soil layer.
type LayerInput struct {
	Name  string  `json:"name"`
	C     float64 `json:"c"`
	Phi   float64 `json:"phi"`
	Gamma float64 `json:"gamma"`
	TopEl float64 `json:"top_el"`
	BotEl float64 `json:"bot_el"`
	Order int     `json:"order"`
}

// AddLayer appends a soil layer to a slope.
func (s *Service) AddLayer(ctx context.Context, slopeID string, in LayerInput) (*model.SoilLayer, error) {
	if err := validateLayer(in); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l := &model.SoilLayer{
		ID: nextID("lyr"), SlopeID: slopeID, Name: in.Name, C: in.C, Phi: in.Phi,
		Gamma: in.Gamma, TopEl: in.TopEl, BotEl: in.BotEl, Order: in.Order,
	}
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		if _, err := s.store.GetSlopeTx(ctx, tx, slopeID); err != nil {
			return err
		}
		if err := s.store.CreateLayer(ctx, tx, l); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: slopeID, Ts: s.now(), Kind: model.EvLayerAdd, Payload: marshalPayload(l),
		})
	})
	if err != nil {
		return nil, err
	}
	return l, nil
}

// ListLayers returns the layers of a slope.
func (s *Service) ListLayers(ctx context.Context, slopeID string) ([]model.SoilLayer, error) {
	if _, err := s.store.GetSlope(ctx, slopeID); err != nil {
		return nil, err
	}
	return s.store.ListLayers(ctx, slopeID)
}

// UpdateLayer patches a layer's parameters.
func (s *Service) UpdateLayer(ctx context.Context, id string, in LayerInput) (*model.SoilLayer, error) {
	if err := validateLayer(in); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var l model.SoilLayer
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		ex, err := s.store.GetLayer(ctx, id)
		if err != nil {
			return err
		}
		l = model.SoilLayer{
			ID: id, SlopeID: ex.SlopeID, Name: in.Name, C: in.C, Phi: in.Phi,
			Gamma: in.Gamma, TopEl: in.TopEl, BotEl: in.BotEl, Order: in.Order,
		}
		if err := s.store.UpdateLayer(ctx, tx, &l); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: ex.SlopeID, Ts: s.now(), Kind: model.EvLayerUpdate, Payload: marshalPayload(l),
		})
	})
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// DeleteLayer removes a layer.
func (s *Service) DeleteLayer(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.InTx(ctx, func(tx *sql.Tx) error {
		ex, err := s.store.GetLayer(ctx, id)
		if err != nil {
			return err
		}
		if err := s.store.DeleteLayer(ctx, tx, id); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: ex.SlopeID, Ts: s.now(), Kind: model.EvLayerDelete, Payload: marshalPayload(map[string]string{"id": id}),
		})
	})
}

func validateLayer(in LayerInput) error {
	if in.C < 0 {
		return fmt.Errorf("%w: cohesion must be >= 0", store.ErrInvariant)
	}
	if in.Phi < 0 || in.Phi > 60 {
		return fmt.Errorf("%w: phi must be in [0,60]", store.ErrInvariant)
	}
	if in.Gamma < 5 || in.Gamma > 30 {
		return fmt.Errorf("%w: gamma must be in [5,30] kN/m³", store.ErrInvariant)
	}
	if in.TopEl <= in.BotEl {
		return fmt.Errorf("%w: top_el must be above bot_el", store.ErrInvariant)
	}
	return nil
}
