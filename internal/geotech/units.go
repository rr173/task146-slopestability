// Package geotech implements the limit-equilibrium slope-stability solver and
// its supporting geometry. It is pure computation: no I/O, no clock, no
// database. Every function here is deterministic and testable in isolation;
// the service layer composes these primitives against persisted inputs.
//
// Conventions:
//   - x is horizontal distance (m), z is elevation (m), positive up.
//   - gamma in kN/m³, c in kPa, phi in degrees, forces in kN, moments kN·m/m.
//   - Circular slip surfaces use a centre (cx, cz) with cz above the crest and
//     the lower arc as the slip; the moment lever arm is the radius R.
//   - Bishop's simplified method iterates F on both sides of the equation;
//     Fellenius is closed-form; Janbu uses a correction factor.
package geotech

import "math"

// Physical constants and solver tolerances.
const (
	GammaW          = 9.81 // kN/m³, unit weight of water
	Epsilon         = 1e-9
	ConvergeTol     = 1e-4 // |ΔF| convergence tolerance
	MaxIter         = 100
	DefaultCapFactor = 1.5 // reinforcement may not raise F above cap_factor×F_baseline
)

// Deg2Rad converts degrees to radians.
func Deg2Rad(d float64) float64 { return d * math.Pi / 180 }

// Rad2Deg converts radians to degrees.
func Rad2Deg(r float64) float64 { return r * 180 / math.Pi }

// Near reports whether two magnitudes are equal within Epsilon.
func Near(a, b float64) bool { return math.Abs(a-b) < Epsilon }

// Clamp pins v into [lo, hi].
func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
