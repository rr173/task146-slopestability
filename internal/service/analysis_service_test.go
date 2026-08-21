package service

import (
	"context"
	"path/filepath"
	"testing"

	"task146-slopestability/internal/clock"
	"task146-slopestability/internal/store"
)

func TestSubmitAnalysisPrefersLatestPiezometerHead(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	svc := NewWithClock(st, clock.NewFake(1000))
	slope, err := svc.CreateSlope(ctx, CreateSlopeInput{Name: "cut", CrestEl: 20, ToeEl: 0, SlopeAngle: 45})
	if err != nil {
		t.Fatal(err)
	}
	for _, layer := range []LayerInput{{Name: "clay", C: 15, Phi: 20, Gamma: 18, TopEl: 20, BotEl: 10, Order: 1}, {Name: "sand", C: 5, Phi: 32, Gamma: 19, TopEl: 10, BotEl: -5, Order: 2}} {
		if _, err := svc.AddLayer(ctx, slope.ID, layer); err != nil {
			t.Fatal(err)
		}
	}
	slip, err := svc.CreateSlipSurface(ctx, slope.ID, SlipInput{Type: "circular", Cx: 5, Cz: 30, Radius: 25})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitAnalysis(ctx, slope.ID, AnalysisInput{SlipSurfaceID: slip.ID, Method: "bishop", SliceCount: 12}); err != nil {
		t.Fatal(err)
	}
	ins, err := svc.CreateInstrument(ctx, slope.ID, InstrumentInput{Type: "piezometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 8, Ts: 1001, Source: "manual"}); err != nil {
		t.Fatal(err)
	}
	analysis, err := svc.SubmitAnalysis(ctx, slope.ID, AnalysisInput{SlipSurfaceID: slip.ID, Method: "bishop", SliceCount: 12, WaterTableEl: 2})
	if err != nil {
		t.Fatal(err)
	}
	if analysis.WaterTableEl != 8 {
		t.Fatalf("analysis water table = %v, want latest piezometer head 8", analysis.WaterTableEl)
	}
}
