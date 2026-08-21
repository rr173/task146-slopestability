package service

import "testing"

func TestBug10_RunWaterTableOverrideIsPersisted(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 2)
	override := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{WaterTableEl: 9})
	if override.WaterTableEl != 9 {
		t.Fatalf("submitted water table = %v, want 9", override.WaterTableEl)
	}
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "inclinometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, online, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 1, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if online != override.FinalF {
		t.Fatalf("online F = %v, want %v", online, override.FinalF)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentF != override.FinalF {
		t.Fatalf("recovered F = %v, want %v", recovered.CurrentF, override.FinalF)
	}
}
