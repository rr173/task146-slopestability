package geotech

import "math"

// reinforcementContribution computes the total tangential resisting force
// (kN/m) available from the given reinforcements, applying the boundary
// constraint that the effective force per element is min(material capacity,
// geotechnical capacity) — a stabilising element cannot deliver more force
// than its weakest link (boundary constraint #3).
//
// Each element's tangential contribution is reduced by cos(angleDeg): a
// horizontal geotextile (angle 0) contributes fully; an inclined anchor
// contributes less along the slip direction. The result is a force; the caller
// converts it to a moment by multiplying by R.
func reinforcementContribution(rs []ReinforcementInput) float64 {
	total := 0.0
	for _, r := range rs {
		material := r.MaterialKN
		geo := r.GeoKN
		if geo > 0 && geo < material {
			material = geo // min(material, geotechnical) — weakest link
		}
		if material <= 0 {
			continue
		}
		angle := Clamp(r.AngleDeg, 0, 89)
		total += material * math.Cos(Deg2Rad(angle))
	}
	return total
}

// ApplyReinforcementCap clamps the reinforced F to cap_factor × baseline so a
// heavily reinforced slope cannot mask a real stability deficit beyond the cap
// (boundary constraint #3, second clause). Returns (f, capped).
func ApplyReinforcementCap(fBaseline, fReinforcedRaw, capFactor float64) (float64, bool) {
	if capFactor <= 0 {
		capFactor = DefaultCapFactor
	}
	fCap := capFactor * fBaseline
	if fReinforcedRaw <= fCap {
		return fReinforcedRaw, false
	}
	return fCap, true
}
