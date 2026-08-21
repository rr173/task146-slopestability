package service

import "testing"

func TestBug01_LivePiezometerOverridesStaticHead(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 2)
	initial := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{WaterTableEl: 2})
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "piezometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, online, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 8, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if online == initial.FinalF {
		t.Fatalf("online F remained %v after the observed head changed", online)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentF != online {
		t.Fatalf("recovered F = %v, want %v", recovered.CurrentF, online)
	}
	latest := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{WaterTableEl: 2})
	if latest.WaterTableEl != 8 {
		t.Fatalf("new analysis water table = %v, want 8", latest.WaterTableEl)
	}
}
