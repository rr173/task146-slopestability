package service

import "testing"

func TestBug09_ZeroPiezometerHeadDriesSlope(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 12)
	initial := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "piezometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, online, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 0, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if online == initial.FinalF {
		t.Fatalf("online F remained %v after the observed head changed", online)
	}
	latest := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	if latest.WaterTableEl != 0 {
		t.Fatalf("new analysis water table = %v, want 0", latest.WaterTableEl)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentF != latest.FinalF {
		t.Fatalf("recovered F = %v, want %v", recovered.CurrentF, latest.FinalF)
	}
}
