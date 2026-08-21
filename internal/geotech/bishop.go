package geotech

import (
	"errors"
	"math"

	"task146-slopestability/internal/model"
)

// ErrNonConverge signals that Bishop iteration did not settle within MaxIter.
var ErrNonConverge = errors.New("geotech: bishop iteration did not converge")

// SolveResult is the output of any method solver.
type SolveResult struct {
	F          float64
	Iterations int
	Converged  bool
	Slices     []model.SliceResult
	Reinforced bool    // true if reinforcement raised the resisting moment
	Capped     bool    // true if the reinforcement cap was hit
	FBaseline  float64 // F without reinforcement (for cap accounting)
}

// SolveInput is the per-run configuration shared by all methods.
type SolveInput struct {
	Profile        Profile
	Layers         []model.SoilLayer
	Cx, Cz, R      float64
	N              int
	WaterTableEl   float64
	MeasuredU      float64 // measured pore pressure at base (kPa); 0 => use water table
	Kh, Kv         float64
	Reinforcements []ReinforcementInput
	CapFactor      float64 // reinforcement cap (0 => DefaultCapFactor)
}

// ReinforcementInput is one stabilising element from the service.
type ReinforcementInput struct {
	Type       string  // "geotextile" | "anchor"
	MaterialKN float64 // design force (tensile or anchor capacity), kN
	GeoKN      float64 // geotechnical (pullout/bond) capacity, kN
	AngleDeg   float64 // orientation (anchor) or 0 (geotextile, horizontal)
}

// SolveBishop runs the Bishop simplified method, iterating F on both sides of
// the equation until |ΔF| < ConvergeTol or MaxIter is reached. Reinforcement is
// applied after a baseline pass (boundary constraint #3: min(material, geotech)
// and a cap_factor×baseline ceiling).
func SolveBishop(in SolveInput) (SolveResult, error) {
	s, err := Slices(in.Profile, in.Layers, in.Cx, in.Cz, in.R, in.N, in.WaterTableEl, in.MeasuredU)
	if err != nil {
		return SolveResult{}, err
	}
	if in.CapFactor <= 0 {
		in.CapFactor = DefaultCapFactor
	}
	// Vertical seismic scales the slice weight in place (kv>0 = downward adds
	// weight); applied before computing terms so it propagates to both sides.
	if in.Kv != 0 {
		for i := range s {
			s[i].WeightW *= (1 + in.Kv)
		}
	}
	terms := computeTerms(s, in.Kh, in.Cz)

	fBase, iter, conv := iterate(s, terms, in.Cz, in.R, in.Profile)
	res := SolveResult{F: fBase, Iterations: iter, Converged: conv, FBaseline: fBase}

	reinf := reinforcementContribution(in.Reinforcements)
	if reinf > Epsilon {
		fRaw, _, _ := iterateWithReinforcement(s, terms, in.Cz, in.R, in.Profile, reinf)
		fCap := in.CapFactor * fBase
		if fRaw <= fCap {
			res.F = fRaw
		} else {
			res.F = fCap
			res.Capped = true
		}
		res.Reinforced = true
	}

	res.Slices = buildSliceResults(s, terms, in.Cz, in.R)
	if !conv {
		return res, ErrNonConverge
	}
	return res, nil
}

// iterate runs the Bishop fixed-point: F_{k+1} = Σ(resist0/m_α(F_k)) / Σdrive.
func iterate(s []Slice, t []sliceTerms, cz, R float64, p Profile) (float64, int, bool) {
	f := 1.0
	for k := 0; k < MaxIter; k++ {
		num := 0.0
		for i := range s {
			ma := mAlpha(s[i], t[i], f)
			if math.Abs(ma) < 1e-6 {
				continue
			}
			num += t[i].resist0 / ma
		}
		den := totalDrive(s, t, cz, R, p)
		if den <= 0 {
			return f, k + 1, false
		}
		fNext := num / den
		if math.Abs(fNext-f) < ConvergeTol {
			return fNext, k + 1, true
		}
		f = fNext
	}
	return f, MaxIter, false
}

// iterateWithReinforcement adds a constant tangential reinforcement moment to
// the resisting numerator. The cap (capping F at cap_factor×baseline) is
// applied by the caller.
func iterateWithReinforcement(s []Slice, t []sliceTerms, cz, R float64, p Profile, reinfMoment float64) (float64, int, bool) {
	f := 1.0
	for k := 0; k < MaxIter; k++ {
		num := 0.0
		for i := range s {
			ma := mAlpha(s[i], t[i], f)
			if math.Abs(ma) < 1e-6 {
				continue
			}
			num += t[i].resist0 / ma
		}
		num += reinfMoment / R
		den := totalDrive(s, t, cz, R, p)
		if den <= 0 {
			return f, k + 1, false
		}
		fNext := num / den
		if math.Abs(fNext-f) < ConvergeTol {
			return fNext, k + 1, true
		}
		f = fNext
	}
	return f, MaxIter, false
}

// mAlpha is the Bishop denominator correction: cos α + sin α·tan φ / F.
func mAlpha(s Slice, t sliceTerms, f float64) float64 {
	return math.Cos(s.Alpha) + math.Sin(s.Alpha)*math.Tan(Deg2Rad(s.Phi))/f
}

// totalDrive sums the driving moment (per R): weight downslope, horizontal
// seismic at the centroid lever, and tension-crack water pressure.
func totalDrive(s []Slice, t []sliceTerms, cz, R float64, p Profile) float64 {
	d := 0.0
	for i := range s {
		d += t[i].drive
		d += t[i].seismicForce * t[i].lever / R
	}
	d += crackDrive(p, cz, R)
	return d
}

// buildSliceResults materialises the per-slice contribution table.
func buildSliceResults(s []Slice, t []sliceTerms, cz, R float64) []model.SliceResult {
	out := make([]model.SliceResult, len(s))
	for i := range s {
		out[i] = model.SliceResult{
			Number: s[i].Number, XM: s[i].XM, ZG: s[i].ZG, ZS: s[i].ZS,
			AlphaDeg: Rad2Deg(s[i].Alpha), WidthB: s[i].WidthB, Height: s[i].Height,
			WeightW: s[i].WeightW, PoreU: s[i].PoreU,
			Cohesion: s[i].C, Phi: s[i].Phi,
			Drive: t[i].drive + t[i].seismicForce*t[i].lever/R,
		}
		ma := mAlpha(s[i], t[i], 1.0)
		if math.Abs(ma) > 1e-6 {
			out[i].Resist = t[i].resist0 / ma
		}
	}
	return out
}
