package service

import "testing"

func TestBug07_RequestedSliceCountControlsOutput(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	coarse := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{SliceCount: 5})
	fine := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{SliceCount: 20})
	if fine.SliceCount != 20 || len(fine.Slices) != 20 {
		t.Fatalf("submitted slices = field %d rows %d, want 20", fine.SliceCount, len(fine.Slices))
	}
	if fine.FinalF == coarse.FinalF {
		t.Fatalf("submitted fine F = %v, want a value different from %v", fine.FinalF, coarse.FinalF)
	}
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "inclinometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, online, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 1, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if online != fine.FinalF {
		t.Fatalf("online F = %v, want %v", online, fine.FinalF)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentF != fine.FinalF {
		t.Fatalf("recovered F = %v, want %v", recovered.CurrentF, fine.FinalF)
	}
}
