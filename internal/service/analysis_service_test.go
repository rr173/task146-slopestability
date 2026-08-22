package service

import (
	"context"
	"path/filepath"
	"testing"

	"task146-slopestability/internal/clock"
	"task146-slopestability/internal/geotech"
	"task146-slopestability/internal/model"
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

// methodF runs a one-off solve purely through the geotech dispatcher so a test
// can ask "what F does method M produce here?" without going through Submit.
func methodF(t *testing.T, svc *Service, ctx context.Context, slopeID, slipID, method string) float64 {
	t.Helper()
	res := runAnalysisForTest(t, svc, ctx, slopeID, slipID, AnalysisInput{Method: method})
	if res.Status != "converged" {
		t.Fatalf("method %s analysis status = %s, want converged", method, res.Status)
	}
	return res.FinalF
}

// TestLiveRecomputeHonoursAnalysisMethod asserts that AddReadingRecord, which
// recomputes F from the last analysis on a new reading, runs the method the
// last analysis was submitted with — not Bishop. A dry slope keeps the pore
// pressure constant so any F gap is purely the method's, not the water table's.
func TestLiveRecomputeHonoursAnalysisMethod(t *testing.T) {
	svc, ctx, slopeID, slipID := configuredAnalysisService(t, 0)
	// Submit the canonical case under Janbu, then under Bishop, on the same
	// surface so we know both solvers converge and differ here.
	bishopF := methodF(t, svc, ctx, slopeID, slipID, "bishop")
	// Make Janbu the last analysis on the slope.
	janbuF := methodF(t, svc, ctx, slopeID, slipID, "janbu")
	if janbuF == bishopF {
		t.Fatalf("janbu F=%v equals bishop F=%v on this surface", janbuF, bishopF)
	}
	// A piezometer reading triggers the live recompute against last.Method=janbu.
	ins, err := svc.CreateInstrument(ctx, slopeID, InstrumentInput{Type: "piezometer", X: 5, InstallEl: 15, RangeMax: 30})
	if err != nil {
		t.Fatal(err)
	}
	sess, recompF, err := svc.AddReadingRecord(ctx, ins.ID, ReadingInput{Value: 0, Ts: 1001, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if sess.RecomputedF != recompF {
		t.Fatalf("session recompF=%v != returned recompF=%v", sess.RecomputedF, recompF)
	}
	if recompF != janbuF {
		t.Fatalf("live recompute F=%v, want janbu F=%v (method not honoured)", recompF, janbuF)
	}
	if recompF == bishopF {
		t.Fatalf("live recompute F=%v equals bishop F=%v (ran Bishop instead of Janbu)", recompF, bishopF)
	}
}

// TestRestartReconcileHonoursAnalysisMethod asserts that the startup reconcile
// path recomputes current_f with the last analysis's method. It seeds a Janbu
// analysis, simulates a crash by closing the store, reopens the same file and
// runs ReconcileAll; the slope's current_f must stay on the Janbu value, not
// drift to Bishop's.
func TestRestartReconcileHonoursAnalysisMethod(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "restart-method.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	svc := NewWithClock(st, clock.NewFake(1000))
	slope, err := svc.CreateSlope(ctx, CreateSlopeInput{Name: "cut", CrestEl: 20, ToeEl: 0, SlopeAngle: 45})
	if err != nil {
		t.Fatal(err)
	}
	for _, layer := range []LayerInput{
		{Name: "clay", C: 15, Phi: 20, Gamma: 18, TopEl: 20, BotEl: 10, Order: 1},
		{Name: "sand", C: 5, Phi: 32, Gamma: 19, TopEl: 10, BotEl: -5, Order: 2},
	} {
		if _, err := svc.AddLayer(ctx, slope.ID, layer); err != nil {
			t.Fatal(err)
		}
	}
	slip, err := svc.CreateSlipSurface(ctx, slope.ID, SlipInput{Type: "circular", Cx: 5, Cz: 30, Radius: 25})
	if err != nil {
		t.Fatal(err)
	}
	janbu := runAnalysisForTest(t, svc, ctx, slope.ID, slip.ID, AnalysisInput{Method: "janbu"})
	before := janbu.FinalF

	// Simulate a crash: close the store, reopen the same file, reconcile.
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	svc2 := NewWithClock(st2, clock.NewFake(1000))
	if _, _, err := svc2.Reconcile().ReconcileAll(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	after, err := svc2.GetSlope(ctx, slope.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !numEqService(before, after.CurrentF) {
		t.Fatalf("current_f changed across restart: before=%v after=%v (method not honoured)", before, after.CurrentF)
	}
	// And confirm it is the Janbu value, not Bishop's, by recomputing both
	// against the same surface and comparing.
	bishopF := geotechF(t, st2, ctx, slope.ID, slip.ID, "bishop")
	if numEqService(after.CurrentF, bishopF) && !numEqService(bishopF, before) {
		t.Fatalf("reconciled current_f=%v equals bishop F=%v but the run was janbu F=%v", after.CurrentF, bishopF, before)
	}
}

// geotechF re-derives a method's F straight from the geotech dispatcher over
// the persisted slope/layer/slip inputs (no service), so the restart test can
// compare the reconciled value against each method's signature independently.
func geotechF(t *testing.T, st *store.Store, ctx context.Context, slopeID, slipID, method string) float64 {
	t.Helper()
	sl, err := st.GetSlope(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	layers, err := st.ListLayers(ctx, slopeID)
	if err != nil {
		t.Fatal(err)
	}
	sf, err := st.GetSlipSurface(ctx, slipID)
	if err != nil {
		t.Fatal(err)
	}
	prof := profileFromSlope(sl)
	gin := geotech.SolveInput{
		Profile: prof, Layers: layers, Cx: sf.Cx, Cz: sf.Cz, R: sf.Radius,
		N: 12, WaterTableEl: sl.WaterTableEl,
	}
	res, err := geotech.Solve(model.AnalysisMethod(method), gin)
	if err != nil {
		t.Fatalf("geotech.Solve(%s): %v", method, err)
	}
	return res.F
}

// numEqService mirrors the smoke tolerance for float comparison.
func numEqService(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-3
}

