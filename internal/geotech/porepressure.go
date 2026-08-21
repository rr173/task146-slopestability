package geotech

import "math"

// resolvePore applies the priority rule: a measured pore pressure (measuredU>0;
// 0 means "not supplied") wins over a water-table-derived pressure, which in
// turn wins over zero. This single rule prevents double-counting pore pressure
// from two sources (boundary constraint #2).
func resolvePore(measuredU, waterTableEl, zs float64) float64 {
	if measuredU > 0 {
		return measuredU
	}
	if waterTableEl > 0 && waterTableEl > zs {
		return GammaW * (waterTableEl - zs)
	}
	return 0
}

// sliceTerms holds the per-slice driving/resisting quantities common to every
// limit-equilibrium method, before the F-in-the-denominator correction.
type sliceTerms struct {
	drive        float64 // W·sin α  (downslope weight component, per R)
	seismicForce float64 // kh·W horizontal inertia force (kN/m)
	lever        float64 // (cz − zBar): moment lever of seismic about centre
	effNorm      float64 // W − u·b   effective normal force (kN/m)
	resist0      float64 // c·b + (W−u·b)·tan φ   shear numerator before m_α
}

// computeTerms derives the per-slice terms. cz is the circle-centre elevation
// used to size the seismic lever arm.
func computeTerms(s []Slice, kh, cz float64) []sliceTerms {
	t := make([]sliceTerms, len(s))
	for i := range s {
		w := s[i].WeightW
		alpha := s[i].Alpha
		t[i].drive = w * math.Sin(alpha)
		t[i].seismicForce = kh * w
		t[i].lever = cz - s[i].ZBar
		if t[i].lever < 0 {
			t[i].lever = 0
		}
		t[i].effNorm = w - s[i].PoreU*s[i].WidthB
		t[i].resist0 = s[i].C*s[i].WidthB + t[i].effNorm*math.Tan(Deg2Rad(s[i].Phi))
	}
	return t
}

// crackDrive is the driving moment (per R) contributed by water pressure in
// the tension crack; lever arms are expressed per R by the caller.
func crackDrive(p Profile, cz, R float64) float64 {
	f := p.crackWaterForce()
	if f <= 0 || R <= 0 {
		return 0
	}
	lever := cz - p.TensionCrackEl
	if lever < 0 {
		lever = 0
	}
	return f * lever / R
}
