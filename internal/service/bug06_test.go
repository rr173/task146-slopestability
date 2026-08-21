package service

import "testing"

func TestBug06_ReinforcementRaisesSafetyFactor(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	plain := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	if _, err := svc.AddReinforcement(ctx, slopeID, ReinforcementInput{Type: "anchor", CapacityKN: 100, AngleDeg: 10, DepthEl: 5}); err != nil {
		t.Fatal(err)
	}
	reinforced := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	if reinforced.FinalF <= plain.FinalF {
		t.Fatalf("submitted reinforcement F = %v, want greater than %v", reinforced.FinalF, plain.FinalF)
	}
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "inclinometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, online, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 1, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if online != reinforced.FinalF {
		t.Fatalf("online F = %v, want %v", online, reinforced.FinalF)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentF != reinforced.FinalF {
		t.Fatalf("recovered F = %v, want %v", recovered.CurrentF, reinforced.FinalF)
	}
}
