package geotech

import "math"

// SolveFellenius runs the Fellenius (ordinary) method. It is non-iterating: F
// is computed closed-form because the normal force is taken as the slice
// weight resolved onto the base (no inter-slice force assumption).
//
//	F = Σ[ c·b + (W·cos α − u·b)·tan φ ] / Σ[ W·sin α + seismic·lever/R + crack ]
func SolveFellenius(in SolveInput) (SolveResult, error) {
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
		num += s[i].C*s[i].WidthB + (w*math.Cos(alpha)-s[i].PoreU*s[i].WidthB)*math.Tan(Deg2Rad(s[i].Phi))
		den += w * math.Sin(alpha)
		den += terms[i].seismicForce * terms[i].lever / in.R
	}
	den += crackDrive(in.Profile, in.Cz, in.R)
	if den <= 0 {
		return SolveResult{Slices: buildSliceResults(s, terms, in.Cz, in.R)}, nil
	}
	fBase := num / den
	res := SolveResult{F: fBase, Converged: true, Iterations: 1, FBaseline: fBase}

	reinf := reinforcementContribution(in.Reinforcements)
	if reinf > Epsilon {
		fRaw := (num + reinf/in.R) / den
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
