package geotech

import (
	"testing"

	"task146-slopestability/internal/model"
)

func TestSolversProduceFiniteAndDistinctSafetyFactors(t *testing.T) {
	profile := Profile{ToeEl: 0, CrestEl: 20, Height: 20, SlopeAngle: 45}
	layers := []model.SoilLayer{
		{C: 15, Phi: 20, Gamma: 18, TopEl: 20, BotEl: 10},
		{C: 5, Phi: 32, Gamma: 19, TopEl: 10, BotEl: -5},
	}
	in := SolveInput{Profile: profile, Layers: layers, Cx: 5, Cz: 30, R: 25, N: 12}
	bishop, err := SolveBishop(in)
	if err != nil || bishop.F <= 0 || len(bishop.Slices) == 0 {
		t.Fatalf("Bishop = %#v, err=%v", bishop, err)
	}
	fellenius, err := SolveFellenius(in)
	if err != nil || fellenius.F <= 0 {
		t.Fatalf("Fellenius = %#v, err=%v", fellenius, err)
	}
	if Near(bishop.F, fellenius.F) {
		t.Fatalf("methods unexpectedly equal: bishop=%v fellenius=%v", bishop.F, fellenius.F)
	}
}
