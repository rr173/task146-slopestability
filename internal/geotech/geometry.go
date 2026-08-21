package geotech

import (
	"errors"
	"math"

	"task146-slopestability/internal/model"
)

// ErrGeometry signals a slip surface that does not intersect the slope to form
// a valid sliding mass (no underground arc, reversed mass, etc.).
var ErrGeometry = errors.New("geotech: illegal slip-surface geometry")

// Profile is the slope cross-section geometry used by the solver.
type Profile struct {
	ToeX       float64 // m, x of the slope toe (origin for the section)
	ToeEl      float64 // m, toe elevation
	CrestEl    float64 // m, crest elevation (= ToeEl + Height)
	Height     float64 // m, crest-toe height
	SlopeAngle float64 // degrees from horizontal (0,90)
	// Surcharge region: uniform load q applied where x >= SurchargeX.
	SurchargeX float64
	SurchargeQ float64
	// TensionCrackEl: elevation of the bottom of a tension crack at the crest;
	// 0 means no crack. The crack removes soil above it on the crest flat and
	// adds a horizontal water force.
	TensionCrackEl float64
}

// TopX returns the horizontal distance from the toe to the crest edge.
func (p Profile) TopX() float64 {
	if p.SlopeAngle <= 0 || p.SlopeAngle >= 90 {
		return p.ToeX + p.Height // vertical slope fallback
	}
	return p.ToeX + p.Height/math.Tan(Deg2Rad(p.SlopeAngle))
}

// GroundEl returns the natural ground elevation at x.
func (p Profile) GroundEl(x float64) float64 {
	topX := p.TopX()
	switch {
	case x <= p.ToeX:
		return p.ToeEl
	case x >= topX:
		return p.CrestEl
	default:
		return p.ToeEl + (x-p.ToeX)*math.Tan(Deg2Rad(p.SlopeAngle))
	}
}

// effectiveGroundEl applies the tension-crack removal: soil above the crack
// bottom on the crest flat is absent, so the slice column starts at the crack
// bottom elevation there.
func (p Profile) effectiveGroundEl(x, zG float64) float64 {
	if p.TensionCrackEl <= 0 {
		return zG
	}
	if zG > p.TensionCrackEl {
		return p.TensionCrackEl
	}
	return zG
}

// crackWaterForce returns the horizontal force (kN/m) from water in the tension
// crack: a triangular hydrostatic distribution from crest down to the crack
// bottom. Zero when there is no crack.
func (p Profile) crackWaterForce() float64 {
	if p.TensionCrackEl <= 0 {
		return 0
	}
	depth := p.CrestEl - p.TensionCrackEl
	if depth <= 0 {
		return 0
	}
	return 0.5 * GammaW * depth * depth
}

// layerAt returns the soil layer whose elevation band contains z (base of slice).
func layerAt(layers []model.SoilLayer, z float64) *model.SoilLayer {
	for i := range layers {
		l := &layers[i]
		if z > l.BotEl-Epsilon && z <= l.TopEl+Epsilon {
			return l
		}
	}
	// fall back to the layer whose band is closest (e.g. z exactly on boundary)
	var best *model.SoilLayer
	bestDist := math.MaxFloat64
	for i := range layers {
		l := &layers[i]
		mid := 0.5 * (l.TopEl + l.BotEl)
		d := math.Abs(z - mid)
		if d < bestDist {
			bestDist = d
			best = l
		}
	}
	return best
}

// Slice is one vertical column of the sliding mass.
type Slice struct {
	Number  int
	XM      float64 // m, centre x
	ZG      float64 // m, effective ground elevation at centre
	ZRawG   float64 // m, raw ground elevation (pre-crack) at centre
	ZS      float64 // m, slip elevation at centre
	Alpha   float64 // radians, base inclination (positive downslope)
	WidthB  float64 // m
	Height  float64 // m, ZG - ZS
	WeightW float64 // kN/m, total slice weight (soil + surcharge)
	PoreU   float64 // kPa, pore pressure at base
	C       float64 // kPa, cohesion at base (layer)
	Phi     float64 // degrees, friction at base (layer)
	ZBar    float64 // m, centroid elevation (ZG+ZS)/2
}

// Slices builds the slice table for a circular slip surface over the profile.
// n is the slice count; the slip arc's underground extent is resolved by
// sampling the arc and finding where it first/last meets the ground.
func Slices(p Profile, layers []model.SoilLayer, cx, cz, R float64, n int, waterTableEl, measuredU float64) ([]Slice, error) {
	if n < 5 {
		n = 5
	}
	if R <= 0 {
		return nil, ErrGeometry
	}
	xStart, xEnd, ok := arcExtent(p, cx, cz, R)
	if !ok {
		return nil, ErrGeometry
	}
	if xEnd <= xStart+Epsilon {
		return nil, ErrGeometry
	}
	b := (xEnd - xStart) / float64(n)
	out := make([]Slice, 0, n)
	for i := 0; i < n; i++ {
		xm := xStart + (float64(i)+0.5)*b
		dx := xm - cx
		if math.Abs(dx) >= R-Epsilon {
			// Outside the arc; skip-embrace: clamp to arc boundary.
			continue
		}
		zs := cz - math.Sqrt(R*R-dx*dx)
		zgRaw := p.GroundEl(xm)
		zgEff := p.effectiveGroundEl(xm, zgRaw)
		if zgEff <= zs+Epsilon {
			// No soil column here (arc above ground) — invalid mass.
			continue
		}
		alpha := math.Asin(Clamp(dx/R, -1, 1))
		h := zgEff - zs
		// Weight = sum of layer unit weights × thickness × width.
		weight := 0.0
		for j := range layers {
			l := &layers[j]
			top := math.Min(l.TopEl, zgEff)
			bot := math.Max(l.BotEl, zs)
			if top > bot+Epsilon {
				weight += l.Gamma * (top - bot) * b
			}
		}
		// Surcharge adds to slices that carry the surface load.
		if p.SurchargeQ > 0 && xm >= p.SurchargeX-Epsilon {
			weight += p.SurchargeQ * b
		}
		// Pore pressure: measured reading wins, else water table, else 0.
		u := resolvePore(measuredU, waterTableEl, zs)
		lb := layerAt(layers, zs)
		var c, phi float64
		if lb != nil {
			c = lb.C
			phi = lb.Phi
		}
		out = append(out, Slice{
			Number: i + 1, XM: xm, ZG: zgEff, ZRawG: zgRaw, ZS: zs,
			Alpha: alpha, WidthB: b, Height: h, WeightW: weight,
			PoreU: u, C: c, Phi: phi, ZBar: 0.5 * (zgEff + zs),
		})
	}
	if len(out) < 3 {
		return nil, ErrGeometry
	}
	return out, nil
}

// arcExtent samples the arc and returns the [xStart, xEnd] range over which the
// slip surface lies below ground (forming the sliding mass). ok=false when no
// valid underground arc exists.
func arcExtent(p Profile, cx, cz, R float64) (float64, float64, bool) {
	const samples = 2000
	lo, hi := cx-R, cx+R
	xStart, xEnd := math.NaN(), math.NaN()
	step := (hi - lo) / float64(samples)
	for i := 0; i <= samples; i++ {
		x := lo + float64(i)*step
		dx := x - cx
		if math.Abs(dx) >= R {
			continue
		}
		zs := cz - math.Sqrt(R*R-dx*dx)
		zg := p.GroundEl(x)
		if zs < zg-Epsilon {
			if math.IsNaN(xStart) {
				xStart = x
			}
			xEnd = x
		}
	}
	if math.IsNaN(xStart) || math.IsNaN(xEnd) {
		return 0, 0, false
	}
	return xStart, xEnd, true
}

// CrackMoment is the horizontal water force in the tension crack times its
// lever arm about the circle centre, added to the driving moment.
func crackMoment(p Profile, cz float64) float64 {
	f := p.crackWaterForce()
	if f <= 0 {
		return 0
	}
	lever := cz - p.TensionCrackEl
	if lever < 0 {
		lever = 0
	}
	return f * lever
}
