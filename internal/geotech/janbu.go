package geotech

import "math"

// SolveJanbu runs the simplified Janbu method with the Janbu correction factor
// f0 (a shape correction based on cohesion/friction ratio and geometry). The
// uncorrected factor is the same form as Fellenius but with forces projected
// horizontally; the correction factor nudges the result toward Bishop-like
// values.
//
//	F0 = Σ[ c·b + (W − u·b)·tan φ ] / Σ[ W·tan α + seismic·lever/R·... + crack ]
//	F  = f0 · F0
func SolveJanbu(in SolveInput) (SolveResult, error) {
	s, err := Slices(in.Profile, in.Layers, in.Cx, in.Cz, in.R, in.N, in.WaterTableEl, in.MeasuredU)
	if err != nil {
		return SolveResult{}, err
	}
	if in.CapFactor <= 0 {
		in.CapFactor = DefaultCapFactor
	}
	if in.Kv != 0 {
		for i := range s {
			s[i].WeightW *= (1 + in.Kv)
		}
	}
	terms := computeTerms(s, in.Kh, in.Cz)

	num := 0.0
	den := 0.0
	for i := range s {
		w := s[i].WeightW
		alpha := s[i].Alpha
		// tan α for the horizontal projection; cos α guards division near zero.
		tanA := math.Tan(alpha)
		if math.Abs(math.Cos(alpha)) < 1e-6 {
			tanA = 0
		}
		num += s[i].C*s[i].WidthB + (w-s[i].PoreU*s[i].WidthB)*math.Tan(Deg2Rad(s[i].Phi))
		den += w * tanA
		den += terms[i].seismicForce * terms[i].lever / in.R
	}
	den += crackDrive(in.Profile, in.Cz, in.R)
	if den <= 0 {
		return SolveResult{Slices: buildSliceResults(s, terms, in.Cz, in.R)}, nil
	}
	f0 := num / den
	f0 *= janbuCorrection(in)
	fBase := f0
	res := SolveResult{F: fBase, Converged: true, Iterations: 1, FBaseline: fBase}

	reinf := reinforcementContribution(in.Reinforcements)
	if reinf > Epsilon {
		fRaw := (num + reinf/in.R) / den * janbuCorrection(in)
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
	return res, nil
}

// janbuCorrection is the simplified correction factor f0 (Hoek & Bray table):
// it depends on the c/(c+σ·tan φ) ratio and a geometry descriptor d/L. We use
// a coarse piecewise approximation sufficient for an engine, not a textbook.
func janbuCorrection(in SolveInput) float64 {
	// d: depth of the deepest point of the arc below the chord; L: chord length.
	// Approximate d ≈ R·(1 − cos(θ/2)) with θ spanning the arc.
	d := 0.25 * in.R
	L := 1.5 * in.R
	if L <= 0 {
		return 1.0
	}
	dOverL := d / L
	// Cohesion fraction from the first layer (coarse proxy).
	cohesive := 0.5
	if len(in.Layers) > 0 {
		l := in.Layers[0]
		if l.C > 0 {
			cohesive = Clamp(l.C/(l.C+10), 0, 1)
		}
	}
	switch {
	case cohesive > 0.8:
		// cohesion-dominated
		if dOverL < 0.2 {
			return 1.05
		}
		return 1.12
	default:
		if dOverL < 0.2 {
			return 1.02
		}
		return 1.08
	}
}
