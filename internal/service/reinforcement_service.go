package service

import (
	"context"
	"database/sql"
	"fmt"

	"task146-slopestability/internal/model"
	"task146-slopestability/internal/store"
)

// --- Reinforcement ---

// ReinforcementInput is the body for creating a reinforcement element.
type ReinforcementInput struct {
	Type       string  `json:"type"`
	TensileKNI float64 `json:"tensile_kni"`
	EmbedTopEl float64 `json:"embed_top_el"`
	EmbedBotEl float64 `json:"embed_bot_el"`
	CapacityKN float64 `json:"capacity_kn"`
	AngleDeg   float64 `json:"angle_deg"`
	DepthEl    float64 `json:"depth_el"`
}

// AddReinforcement stores a stabilising element.
func (s *Service) AddReinforcement(ctx context.Context, slopeID string, in ReinforcementInput) (*model.Reinforcement, error) {
	if err := validateReinforcement(in); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := &model.Reinforcement{
		ID: nextID("rnf"), SlopeID: slopeID, Type: model.ReinforcementType(in.Type),
		TensileKNI: in.TensileKNI, EmbedTopEl: in.EmbedTopEl, EmbedBotEl: in.EmbedBotEl,
		CapacityKN: in.CapacityKN, AngleDeg: in.AngleDeg, DepthEl: in.DepthEl,
		CreatedAt: s.now(),
	}
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		if _, err := s.store.GetSlopeTx(ctx, tx, slopeID); err != nil {
			return err
		}
		// Store ComputedEff as a placeholder (recomputed on reconcile/analysis).
		return s.store.CreateReinforcement(ctx, tx, r)
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}

// ListReinforcements returns the reinforcements for a slope.
func (s *Service) ListReinforcements(ctx context.Context, slopeID string) ([]model.Reinforcement, error) {
	if _, err := s.store.GetSlope(ctx, slopeID); err != nil {
		return nil, err
	}
	return s.store.ListReinforcements(ctx, slopeID)
}

// DeleteReinforcement removes a reinforcement element.
func (s *Service) DeleteReinforcement(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.InTx(ctx, func(tx *sql.Tx) error {
		return s.store.DeleteReinforcement(ctx, tx, id)
	})
}

func validateReinforcement(in ReinforcementInput) error {
	t := model.ReinforcementType(in.Type)
	if t != model.ReinfGeotextile && t != model.ReinfAnchor {
		return fmt.Errorf("%w: unknown reinforcement type", store.ErrInvariant)
	}
	if t == model.ReinfGeotextile {
		if in.TensileKNI <= 0 {
			return fmt.Errorf("%w: tensile_kni must be > 0", store.ErrInvariant)
		}
		if in.EmbedTopEl <= in.EmbedBotEl {
			return fmt.Errorf("%w: embed_top_el must be above embed_bot_el", store.ErrInvariant)
		}
	}
	if t == model.ReinfAnchor {
		if in.CapacityKN <= 0 {
			return fmt.Errorf("%w: capacity_kn must be > 0", store.ErrInvariant)
		}
		if in.AngleDeg < 0 || in.AngleDeg >= 90 {
			return fmt.Errorf("%w: angle_deg must be in [0,90)", store.ErrInvariant)
		}
	}
	return nil
}
