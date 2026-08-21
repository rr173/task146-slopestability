package geotech

import "testing"

func TestReinforcementUsesWeakestCapacityAndCap(t *testing.T) {
	got := reinforcementContribution([]ReinforcementInput{{MaterialKN: 100, GeoKN: 20, AngleDeg: 0}})
	if !Near(got, 20) {
		t.Fatalf("reinforcement contribution = %v, want 20", got)
	}
	f, capped := ApplyReinforcementCap(1.2, 2.4, 1.5)
	if !capped || !Near(f, 1.8) {
		t.Fatalf("cap result = (%v, %v), want (1.8, true)", f, capped)
	}
}
