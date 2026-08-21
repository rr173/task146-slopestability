package geotech

import "testing"

func TestResolvePoreHonoursMeasurementThenWaterTable(t *testing.T) {
	if got := resolvePore(37, 20, 5); got != 37 {
		t.Fatalf("measured pore pressure = %v, want 37", got)
	}
	if got, want := resolvePore(0, 12, 5), GammaW*7; !Near(got, want) {
		t.Fatalf("water-table pore pressure = %v, want %v", got, want)
	}
	if got := resolvePore(0, 4, 5); got != 0 {
		t.Fatalf("dry base pressure = %v, want 0", got)
	}
}
