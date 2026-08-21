package service

import "testing"

func TestBug05_TensionCrackAffectsSafetyFactor(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	plain := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	cracked := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{TensionCrackDepth: 5})
	if cracked.FinalF == plain.FinalF {
		t.Fatalf("submitted cracked F = %v, want a value different from %v", cracked.FinalF, plain.FinalF)
	}
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "inclinometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, online, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 1, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if online != cracked.FinalF {
		t.Fatalf("online F = %v, want %v", online, cracked.FinalF)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentF != cracked.FinalF {
		t.Fatalf("recovered F = %v, want %v", recovered.CurrentF, cracked.FinalF)
	}
}
