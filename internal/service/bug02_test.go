package service

import "testing"

func TestBug02_HorizontalSeismicChangesSafetyFactor(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	still := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	seismic := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Kh: 0.2})
	if seismic.FinalF == still.FinalF {
		t.Fatalf("submitted seismic F = %v, want a value different from %v", seismic.FinalF, still.FinalF)
	}
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "inclinometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, online, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 1, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if online != seismic.FinalF {
		t.Fatalf("online F = %v, want %v", online, seismic.FinalF)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentF != seismic.FinalF {
		t.Fatalf("recovered F = %v, want %v", recovered.CurrentF, seismic.FinalF)
	}
}
