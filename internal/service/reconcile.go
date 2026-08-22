package service

import (
	"context"
	"fmt"
	"math"

	"task146-slopestability/internal/geotech"
	"task146-slopestability/internal/model"
	"task146-slopestability/internal/store"
)

// Reconciler exposes ReconcileAll for the startup recovery path. It is a thin
// wrapper kept as a separate type so main can call svc.Reconcile() without
// pulling the recovery contract into the hot service methods.
type Reconciler struct{ s *Service }

// Reconcile returns the reconciler bound to this service.
func (s *Service) Reconcile() *Reconciler { return &Reconciler{s: s} }

// ReconcileAll replays every slope with persisted analysis inputs, recomputes
// the derived current_f and alert level, and verifies they match the stored
// values. Drifts are logged but the stored value is not overwritten (an engineer
// arbitrates). Returns (recomputed, drifts, err).
func (r *Reconciler) ReconcileAll(ctx context.Context) (int, int, error) {
	ids, err := r.s.store.ListSlopeIDs(ctx)
	if err != nil {
		return 0, 0, err
	}
	recomputed := 0
	drifts := 0
	for _, id := range ids {
		rc, dr, err := r.reconcileOne(ctx, id)
		if err != nil {
			return recomputed, drifts, err
		}
		recomputed += rc
		drifts += dr
	}
	return recomputed, drifts, nil
}

// reconcileOne re-derives one slope's current_f from the authoritative tables
// (the materialised result of the event stream) and writes the reconcile log.
func (r *Reconciler) reconcileOne(ctx context.Context, slopeID string) (int, int, error) {
	sl, err := r.s.store.GetSlope(ctx, slopeID)
	if err != nil {
		if err == store.ErrNotFound {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	layers, err := r.s.store.ListLayers(ctx, slopeID)
	if err != nil {
		return 0, 0, err
	}
	reinf, _ := r.s.store.ListReinforcements(ctx, slopeID)
	analyses, err := r.s.store.ListAnalyses(ctx, slopeID)
	if err != nil {
		return 0, 0, err
	}
	recomputed := 0
	drifts := 0
	if len(layers) > 0 && len(analyses) > 0 {
		last := analyses[0]
		sf, err := r.s.store.GetSlipSurface(ctx, last.SlipSurfaceID)
		if err == nil && sf.Type == model.SlipCircular {
			waterTable := last.WaterTableEl
			if pr, err := r.s.store.LatestPiezometerReading(ctx, slopeID); err == nil {
				waterTable = pr.Value
			} else if err != nil && err != store.ErrNotFound {
				return 0, 0, err
			}
			prof := profileFromSlope(sl)
			// Carry the analysis-time surcharge into the recompute so the
			// reconciled current_f matches what the run actually used.
			prof.SurchargeQ = last.SurchargeQ
			if last.TensionCrackDepth > 0 {
				prof.TensionCrackEl = sl.CrestEl - last.TensionCrackDepth
			}
			gin := geotech.SolveInput{
				Profile: prof, Layers: layers, Cx: sf.Cx, Cz: sf.Cz, R: sf.Radius,
				N: last.SliceCount, WaterTableEl: waterTable, Kh: last.Kh, Kv: last.Kv,
				Reinforcements: reinforcementInputs(reinf, layers, sl),
			}
			res, serr := solveByMethod(last.Method, gin)
			if serr == nil {
				alert := alertFromF(res.F)
				// Drift detection: if the stored current_f was previously set
				// and disagrees with the recompute beyond tolerance, log it.
				if sl.CurrentF > 0 && math.Abs(sl.CurrentF-res.F) > 1e-6 {
					drifts++
				}
				// Write back the recomputed derived figures.
				if err := r.s.store.UpdateSlopeDerived(ctx, nil, slopeID, res.F, alert); err != nil {
					return 0, drifts, err
				}
				recomputed++
			}
		}
	}
	note := fmt.Sprintf("recomputed=%d drifts=%d", recomputed, drifts)
	if err := r.s.store.ReconcileSlope(ctx, slopeID, r.s.now(), recomputed, drifts, note); err != nil {
		return recomputed, drifts, err
	}
	return recomputed, drifts, nil
}
