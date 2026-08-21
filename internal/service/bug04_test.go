package service

import "testing"

func TestBug04_RunSurchargeChangesSafetyFactor(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	plain := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	loaded := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{SurchargeQ: 50})
	if loaded.FinalF == plain.FinalF {
		t.Fatalf("submitted loaded F = %v, want a value different from %v", loaded.FinalF, plain.FinalF)
	}
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "inclinometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, online, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 1, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if online != loaded.FinalF {
		t.Fatalf("online F = %v, want %v", online, loaded.FinalF)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentF != loaded.FinalF {
		t.Fatalf("recovered F = %v, want %v", recovered.CurrentF, loaded.FinalF)
	}
}
