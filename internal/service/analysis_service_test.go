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

func configuredAnalysisService(t *testing.T, staticWater float64) (*Service, context.Context, string, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "service.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	svc := NewWithClock(st, clock.NewFake(1000))
	slope, err := svc.CreateSlope(ctx, CreateSlopeInput{Name: "cut", CrestEl: 20, ToeEl: 0, SlopeAngle: 45, WaterTableEl: staticWater})
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
	return svc, ctx, slope.ID, slip.ID
}

func runAnalysisForTest(t *testing.T, svc *Service, ctx context.Context, slopeID, slipID string, in AnalysisInput) *AnalysisResult {
	t.Helper()
	in.SlipSurfaceID = slipID
	if in.Method == "" {
		in.Method = "bishop"
	}
	if in.SliceCount == 0 {
		in.SliceCount = 12
	}
	result, err := svc.SubmitAnalysis(ctx, slopeID, in)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAnalysisHonoursHorizontalSeismicInput(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	still := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	seismic := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Kh: 0.2})
	if seismic.FinalF == still.FinalF {
		t.Fatalf("horizontal seismic input did not change F: %v", seismic.FinalF)
	}
}

func TestAnalysisHonoursVerticalSeismicInput(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	still := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	seismic := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Kv: 0.2})
	if seismic.FinalF == still.FinalF {
		t.Fatalf("vertical seismic input did not change F: %v", seismic.FinalF)
	}
}

// TestMonitoringRecomputeHonoursVerticalSeismic confirms that a monitoring
// reading replay the last analysis with its persisted Kv, so the recomputed F
// still reflects the configured vertical seismic action.
func TestMonitoringRecomputeHonoursVerticalSeismic(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	still := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	seismic := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Kv: 0.2})
	if seismic.FinalF == still.FinalF {
		t.Fatalf("vertical seismic input did not change F: %v", seismic.FinalF)
	}
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "piezometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	_, recompF, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 8, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if recompF == still.FinalF {
		t.Fatalf("monitoring recompute dropped vertical seismic: recompF=%v, still=%v", recompF, still.FinalF)
	}
}

// TestReconcileHonoursVerticalSeismic confirms that the restart reconciliation
// path reuses the persisted Kv when re-deriving current_f, so the configured
// vertical seismic action survives a service restart.
func TestReconcileHonoursVerticalSeismic(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	still := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	seismic := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Kv: 0.2})
	if seismic.FinalF == still.FinalF {
		t.Fatalf("vertical seismic input did not change F: %v", seismic.FinalF)
	}
	if _, _, err := svc.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	sl, err := svc.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	if sl.CurrentF == still.FinalF {
		t.Fatalf("reconcile dropped vertical seismic: current_f=%v, still=%v", sl.CurrentF, still.FinalF)
	}
}

func TestAnalysisAppliesRunSurcharge(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	plain := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	loaded := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{SurchargeQ: 50})
	if loaded.FinalF == plain.FinalF {
		t.Fatalf("surcharge did not change F: %v", loaded.FinalF)
	}
}

func TestAnalysisAppliesTensionCrackDepth(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	plain := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	cracked := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{TensionCrackDepth: 5})
	if cracked.FinalF == plain.FinalF {
		t.Fatalf("tension crack did not change F: %v", cracked.FinalF)
	}
}

func TestAnalysisIncludesReinforcement(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	plain := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	if _, err := svc.AddReinforcement(ctx, slopeID, ReinforcementInput{Type: "anchor", CapacityKN: 100, AngleDeg: 10, DepthEl: 5}); err != nil {
		t.Fatal(err)
	}
	reinforced := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	if reinforced.FinalF <= plain.FinalF {
		t.Fatalf("reinforcement F=%v, want greater than %v", reinforced.FinalF, plain.FinalF)
	}
}

func TestAnalysisUsesRequestedSliceCount(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	result := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{SliceCount: 20})
	if result.SliceCount != 20 || len(result.Slices) != 20 {
		t.Fatalf("slice result count = field %d, rows %d; want 20", result.SliceCount, len(result.Slices))
	}
}

func TestAnalysisKeepsRequestedMethod(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	bishop := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Method: "bishop"})
	janbu := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Method: "janbu"})
	if janbu.Method != "janbu" || janbu.FinalF == bishop.FinalF {
		t.Fatalf("Janbu analysis = method %q F %v; bishop F=%v", janbu.Method, janbu.FinalF, bishop.FinalF)
	}
}

func TestZeroPiezometerHeadOverridesStaticWater(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 12)
	_ = runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "piezometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 0, Ts: 1001, Source: "manual"}); err != nil {
		t.Fatal(err)
	}
	result := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{})
	if result.WaterTableEl != 0 {
		t.Fatalf("analysis water table = %v, want measured dry head 0", result.WaterTableEl)
	}
}

func TestAnalysisRetainsRunWaterTableOverride(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 2)
	result := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{WaterTableEl: 9})
	if result.WaterTableEl != 9 {
		t.Fatalf("analysis water table = %v, want explicit run head 9", result.WaterTableEl)
	}
}
