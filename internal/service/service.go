// Package service orchestrates the slope-stability engine: it owns the
// lifecycle state machine, the atomic monitoring-recompute transaction, the
// compliance verdicts and the restart-reconciliation replay. It composes the
// pure geotech solver against the persisted authoritative inputs.
//
// The service mutex serialises state mutations. It is NOT re-entrant: methods
// that hold the lock must call internal no-lock helpers rather than re-entering
// a locking public method (otherwise a read that re-runs recompute while the
// outer call still holds the lock would self-deadlock).
package service

import (
	"math"
	"sync"

	"task146-slopestability/internal/clock"
	"task146-slopestability/internal/geotech"
	"task146-slopestability/internal/idlib"
	"task146-slopestability/internal/model"
	"task146-slopestability/internal/store"
)

// Required safety-factor thresholds by scenario. The static curve drives the
// alert level; the others are reported as compliance verdicts.
const (
	RequiredStatic    = 1.5
	RequiredTransient = 1.3
	RequiredSeismic   = 1.1
	AlertYellowBand   = 0.9 // F between 0.9×required and required => yellow

	InclinometerRedDispMM = 10.0 // mm cumulative displacement triggers red
	PorePressureRedDelta  = 20.0 // kPa rise within a 6h window triggers red
	PorePressureWindowS   = 6 * 3600
)

// Service is the single business entry point.
type Service struct {
	store *store.Store
	clk   clock.Clock
	mu    sync.Mutex
}

// New builds a Service with the real clock.
func New(st *store.Store) *Service { return &Service{store: st, clk: clock.Real{}} }

// NewWithClock builds a Service with an injected clock (used by the selfcheck).
func NewWithClock(st *store.Store, clk clock.Clock) *Service {
	return &Service{store: st, clk: clk}
}

// Store exposes the store for the selfcheck (read-only).
func (s *Service) Store() *store.Store { return s.store }

func (s *Service) now() int64 { return s.clk.Epoch() }

// profileFromSlope converts the stored slope into the solver's Profile.
func profileFromSlope(sl *model.Slope) geotech.Profile {
	return geotech.Profile{
		ToeX: 0, ToeEl: sl.ToeEl, CrestEl: sl.CrestEl, Height: sl.Height,
		SlopeAngle: sl.SlopeAngle, SurchargeX: sl.SurchargeX, SurchargeQ: sl.SurchargeQ,
		TensionCrackEl: sl.TensionCrackEl,
	}
}

// layerAtElev returns the soil layer whose band contains el.
func layerAtElev(layers []model.SoilLayer, el float64) model.SoilLayer {
	var best model.SoilLayer
	bestDist := math.MaxFloat64
	for _, l := range layers {
		if el > l.BotEl-1e-9 && el <= l.TopEl+1e-9 {
			return l
		}
		d := math.Abs(el - 0.5*(l.TopEl+l.BotEl))
		if d < bestDist {
			bestDist = d
			best = l
		}
	}
	return best
}

// overburden returns the total vertical stress (kPa) at elevation el, summed
// over the layers above el using their unit weights.
func overburden(layers []model.SoilLayer, sl *model.Slope, el float64) float64 {
	sigma := 0.0
	for _, l := range layers {
		top := l.TopEl
		bot := math.Max(l.BotEl, el)
		if top > bot {
			sigma += l.Gamma * (top - bot)
		}
	}
	return sigma
}

// effectiveReinforcement computes the (material, geotechnical, angle) tuple for
// one reinforcement element. The geotechnical capacity is the pullout/bond
// resistance derived from the surrounding soil, so the solver's min(material,
// geotech) rule honours the weakest-link constraint.
func effectiveReinforcement(r model.Reinforcement, layers []model.SoilLayer, sl *model.Slope) (material, geo, angle float64) {
	switch r.Type {
	case model.ReinfGeotextile:
		material = r.TensileKNI
		embed := r.EmbedTopEl - r.EmbedBotEl
		midEl := 0.5 * (r.EmbedTopEl + r.EmbedBotEl)
		l := layerAtElev(layers, midEl)
		sigmaV := overburden(layers, sl, midEl)
		// Pullout resistance acts on both faces of the embedded strip.
		geo = 2 * sigmaV * math.Tan(geotech.Deg2Rad(l.Phi)) * embed
		angle = 0
	case model.ReinfAnchor:
		material = r.CapacityKN
		l := layerAtElev(layers, r.DepthEl)
		sigmaV := overburden(layers, sl, r.DepthEl)
		// Bond strength along a fixed bond length.
		const bondLength = 3.0
		geo = (l.C + sigmaV*math.Tan(geotech.Deg2Rad(l.Phi))) * bondLength
		angle = r.AngleDeg
	}
	return material, geo, angle
}

// reinforcementInputs converts the stored reinforcements into solver inputs.
func reinforcementInputs(rs []model.Reinforcement, layers []model.SoilLayer, sl *model.Slope) []geotech.ReinforcementInput {
	out := make([]geotech.ReinforcementInput, 0, len(rs))
	for _, r := range rs {
		material, geo, angle := effectiveReinforcement(r, layers, sl)
		out = append(out, geotech.ReinforcementInput{
			Type: string(r.Type), MaterialKN: material, GeoKN: geo, AngleDeg: angle,
		})
	}
	return out
}

// alertFromF maps a recomputed safety factor to an alert level against the
// static required curve. F >= required => green; F >= 0.9×required => yellow;
// else red. Additional red triggers (inclinometer displacement, pore-pressure
// surge) are layered on by the monitoring service.
func alertFromF(f float64) model.AlertLevel {
	if f <= 0 {
		return model.AlertGreen
	}
	switch {
	case f >= RequiredStatic:
		return model.AlertGreen
	case f >= AlertYellowBand*RequiredStatic:
		return model.AlertYellow
	default:
		return model.AlertRed
	}
}

// nextID wraps idlib.New for brevity in the method files.
func nextID(prefix string) string { return idlib.New(prefix) }
