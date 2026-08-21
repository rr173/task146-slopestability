package service

import (
	"context"
	"database/sql"
	"fmt"

	"task146-slopestability/internal/model"
)

// --- Compliance ---

// ComplianceSummary aggregates the verdicts for the three scenarios.
type ComplianceSummary struct {
	SlopeID  string             `json:"slope_id"`
	CurrentF float64            `json:"current_f"`
	Verdicts []model.Compliance `json:"verdicts"`
}

// GetCompliance re-reads the three-scenario compliance verdicts for a slope
// from its current_f and returns them.
func (s *Service) GetCompliance(ctx context.Context, slopeID string) (*ComplianceSummary, error) {
	sl, err := s.store.GetSlope(ctx, slopeID)
	if err != nil {
		return nil, err
	}
	verdicts := []model.Compliance{
		makeCompliance(nextID("cmp"), slopeID, "", model.ScenarioStatic, RequiredStatic, sl.CurrentF),
		makeCompliance(nextID("cmp"), slopeID, "", model.ScenarioTransient, RequiredTransient, sl.CurrentF),
		makeCompliance(nextID("cmp"), slopeID, "", model.ScenarioSeismic, RequiredSeismic, sl.CurrentF),
	}
	return &ComplianceSummary{SlopeID: slopeID, CurrentF: sl.CurrentF, Verdicts: verdicts}, nil
}

// writeComplianceTx rewrites the three-scenario compliance rows for a slope
// inside the caller's transaction. Old rows are removed so the latest verdict
// is authoritative.
func (s *Service) writeComplianceTx(ctx context.Context, tx *sql.Tx, slopeID, sessionID string, f float64) error {
	if err := s.store.DeleteComplianceForSlope(ctx, tx, slopeID); err != nil {
		return err
	}
	for _, scenario := range []model.ComplianceScenario{model.ScenarioStatic, model.ScenarioTransient, model.ScenarioSeismic} {
		req := RequiredStatic
		switch scenario {
		case model.ScenarioTransient:
			req = RequiredTransient
		case model.ScenarioSeismic:
			req = RequiredSeismic
		}
		c := makeCompliance(nextID("cmp"), slopeID, sessionID, scenario, req, f)
		if err := s.store.CreateCompliance(ctx, tx, &c); err != nil {
			return err
		}
	}
	return nil
}

func makeCompliance(id, slopeID, sessionID string, scenario model.ComplianceScenario, required, actual float64) model.Compliance {
	verdict := "fail"
	detail := fmt.Sprintf("actual %.3f < required %.3f", actual, required)
	if actual >= required {
		verdict = "pass"
		detail = fmt.Sprintf("actual %.3f >= required %.3f", actual, required)
	}
	return model.Compliance{
		ID: id, SlopeID: slopeID, SessionID: sessionID, Scenario: scenario,
		RequiredF: required, ActualF: actual, Verdict: verdict, Detail: detail,
	}
}
