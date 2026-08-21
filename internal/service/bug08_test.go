package service

import "testing"

func TestBug08_RequestedMethodChangesSolver(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	bishop := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Method: "bishop"})
	janbu := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Method: "janbu"})
	if janbu.Method != "janbu" || janbu.FinalF == bishop.FinalF {
		t.Fatalf("submitted Janbu result = method %q F %v; bishop F=%v", janbu.Method, janbu.FinalF, bishop.FinalF)
	}
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "inclinometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, online, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 1, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if online != janbu.FinalF {
		t.Fatalf("online F = %v, want %v", online, janbu.FinalF)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentF != janbu.FinalF {
		t.Fatalf("recovered F = %v, want %v", recovered.CurrentF, janbu.FinalF)
	}
}
