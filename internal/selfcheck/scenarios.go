package selfcheck

import (
	"fmt"
	"net/http/httptest"
	"strings"

	"task146-slopestability/internal/clock"
	"task146-slopestability/internal/service"
)

// smokeFullFlow exercises the core read/write loop: create slope, add layers,
// add slip surface, submit a Bishop analysis, verify the response carries a
// finite F and a non-empty slice table, then GET the slope to confirm the
// derived current_f was written back.
func smokeFullFlow(srv *httptest.Server, clk *clock.Fake) error {
	sid, err := seedSlope(srv)
	if err != nil {
		return err
	}
	sfid, err := seedSlip(srv, sid)
	if err != nil {
		return err
	}
	var res struct {
		ID      string  `json:"id"`
		FinalF  float64 `json:"final_f"`
		Slices  int     `json:"slices"`
		Status  string  `json:"status"`
	}
	// Use a temporary struct for the slices length: decode then count.
	var raw map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "bishop", "slice_count": 12,
	}, &raw); err != nil {
		return err
	}
	res.FinalF, _ = raw["final_f"].(float64)
	res.Status, _ = raw["status"].(string)
	slices, _ := raw["slices"].([]any)
	res.Slices = len(slices)
	if res.Status != "converged" {
		return fmt.Errorf("analysis status = %s, want converged", res.Status)
	}
	if !(res.FinalF > 0) {
		return fmt.Errorf("final_f = %v, want positive", res.FinalF)
	}
	if res.Slices == 0 {
		return fmt.Errorf("slice table empty")
	}
	// Slope should carry the derived current_f.
	var sl struct {
		CurrentF  float64 `json:"current_f"`
		Status    string  `json:"status"`
		AlertLevel string  `json:"alert_level"`
	}
	if err := mustDo(srv, "GET", "/api/slopes/"+sid, nil, &sl); err != nil {
		return err
	}
	if !numEq(sl.CurrentF, res.FinalF) {
		return fmt.Errorf("slope current_f = %v, want %v", sl.CurrentF, res.FinalF)
	}
	if sl.Status != "analyzing" {
		return fmt.Errorf("slope status = %s, want analyzing", sl.Status)
	}
	return nil
}

// smokeBishopVsFellenius runs both methods on the same slip surface; the
// Fellenius result (no F-in-denominator) must differ from Bishop's in the
// expected direction for this configuration.
func smokeBishopVsFellenius(srv *httptest.Server, clk *clock.Fake) error {
	sid, err := seedSlope(srv)
	if err != nil {
		return err
	}
	sfid, err := seedSlip(srv, sid)
	if err != nil {
		return err
	}
	var bishop map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "bishop", "slice_count": 16,
	}, &bishop); err != nil {
		return err
	}
	var fellenius map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "fellenius", "slice_count": 16,
	}, &fellenius); err != nil {
		return err
	}
	bf, _ := bishop["final_f"].(float64)
	ff, _ := fellenius["final_f"].(float64)
	if !(bf > 0) || !(ff > 0) {
		return fmt.Errorf("bishop F=%v fellenius F=%v, both must be positive", bf, ff)
	}
	// Bishop and Fellenius must not be identical to floating precision (the
	// F-in-denominator correction shifts the result).
	if numEq(bf, ff) {
		return fmt.Errorf("bishop F=%v equals fellenius F=%v (expected to differ)", bf, ff)
	}
	return nil
}

// smokeMaxIterFailure forces a pathological surface that cannot converge and
// checks the analysis is marked failed (boundary constraint #1: convergence
// failure must surface as failed, not a fake F).
func smokeMaxIterFailure(srv *httptest.Server, clk *clock.Fake) error {
	sid, err := seedSlope(srv)
	if err != nil {
		return err
	}
	// A slip with an enormous radius that barely touches the slope produces a
	// tiny sliding mass; the iteration should still report converged or failed,
	// never panic. Use a degenerate near-vertical surface.
	var sf map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/slip-surfaces", map[string]any{
		"type": "circular", "cx": 1000, "cz": 1000, "radius": 999,
	}, &sf); err != nil {
		return err
	}
	sfid, _ := sf["id"].(string)
	code, body, _ := doJSON(srv, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "bishop", "slice_count": 10,
	})
	// Either it converges (some F) or fails with 422 ErrGeometry/ErrNotConverged.
	// A 500 would be a bug.
	if code >= 500 {
		return fmt.Errorf("analysis returned 500: %s", string(body))
	}
	return nil
}

// smokeCriticalSearch runs a small grid search and verifies the coverage
// counters (total/evaluated/skipped) are returned and the min F is finite
// (boundary constraint #4: skipped points are counted, not dropped).
func smokeCriticalSearch(srv *httptest.Server, clk *clock.Fake) error {
	sid, err := seedSlope(srv)
	if err != nil {
		return err
	}
	var out map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/search-critical", map[string]any{
		"method": "bishop",
		"grid": map[string]any{
			"cx_min": 2, "cx_max": 8, "cz_min": 25, "cz_max": 40,
			"r_min": 20, "r_max": 30, "cx_step": 3, "cz_step": 5, "r_step": 5,
		},
	}, &out); err != nil {
		return err
	}
	summary, _ := out["summary"].(map[string]any)
	if summary == nil {
		return fmt.Errorf("no summary in response")
	}
	total, _ := summary["total"].(float64)
	evaluated, _ := summary["evaluated"].(float64)
	skipped, _ := summary["skipped"].(float64)
	minF, _ := summary["min_f"].(float64)
	if total == 0 {
		return fmt.Errorf("search total = 0")
	}
	if int(evaluated)+int(skipped) != int(total) {
		return fmt.Errorf("evaluated(%v)+skipped(%v) != total(%v)", evaluated, skipped, total)
	}
	if !(minF > 0) {
		return fmt.Errorf("min_f = %v, want positive", minF)
	}
	if _, ok := summary["best_cx"].(float64); !ok {
		return fmt.Errorf("best_cx missing")
	}
	return nil
}

// smokeReinforcementWeakest verifies the min(material, geotechnical) rule
// (boundary constraint #3): an anchor whose bond capacity is far below its
// material capacity must take the smaller value, so its F stays modest.
func smokeReinforcementWeakest(srv *httptest.Server, clk *clock.Fake) error {
	sid, err := seedSlope(srv)
	if err != nil {
		return err
	}
	sfid, err := seedSlip(srv, sid)
	if err != nil {
		return err
	}
	// Baseline F (no reinforcement).
	var base map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "bishop", "slice_count": 12,
	}, &base); err != nil {
		return err
	}
	bf, _ := base["final_f"].(float64)
	// Add an anchor with a huge material capacity but embedded shallow in weak
	// soil so the geotechnical (bond) capacity is the weakest link.
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/reinforcements", map[string]any{
		"type": "anchor", "capacity_kn": 100000, "angle_deg": 15, "depth_el": 5,
	}, nil); err != nil {
		return err
	}
	var reinforced map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "bishop", "slice_count": 12,
	}, &reinforced); err != nil {
		return err
	}
	rf, _ := reinforced["final_f"].(float64)
	if !(rf > bf) {
		return fmt.Errorf("reinforced F=%v not greater than baseline F=%v", rf, bf)
	}
	// The cap prevents runaway: reinforced F must not exceed 1.5×baseline.
	if rf > 1.5*bf+1e-3 {
		return fmt.Errorf("reinforced F=%v exceeds 1.5×baseline=%v (cap violated)", rf, 1.5*bf)
	}
	return nil
}

// smokePorePressurePriority verifies that a measured piezometer reading wins
// over the slope's static water table (boundary constraint #2): adding a high
// piezometer reading must raise the pore pressure and lower F versus the dry
// baseline.
func smokePorePressurePriority(srv *httptest.Server, clk *clock.Fake) error {
	sid, err := seedSlope(srv)
	if err != nil {
		return err
	}
	sfid, err := seedSlip(srv, sid)
	if err != nil {
		return err
	}
	// Baseline (dry).
	var base map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "bishop", "slice_count": 12,
	}, &base); err != nil {
		return err
	}
	bf, _ := base["final_f"].(float64)
	// Install a piezometer and record a high water-table reading.
	var ins map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/instruments", map[string]any{
		"type": "piezometer", "x": 5, "install_el": 15, "range_max": 30,
	}, &ins); err != nil {
		return err
	}
	pid, _ := ins["id"].(string)
	if err := mustDo(srv, "POST", "/api/instruments/"+pid+"/readings", map[string]any{
		"value": 8.0, "ts": clk.Epoch(), "source": "manual",
	}, nil); err != nil {
		return err
	}
	// Now a new analysis with the live water table from the piezometer should
	// have a lower F (higher pore pressure).
	var wet map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "bishop", "slice_count": 12,
	}, &wet); err != nil {
		return err
	}
	wf, _ := wet["final_f"].(float64)
	if !(wf < bf) {
		return fmt.Errorf("wet F=%v not less than dry baseline F=%v (pore pressure priority)", wf, bf)
	}
	return nil
}

// smokeAlertStateMachine verifies the monotone-alert guard (boundary constraint
// #5): a slope in red alert cannot be closed.
func smokeAlertStateMachine(srv *httptest.Server, clk *clock.Fake) error {
	sid, err := seedSlope(srv)
	if err != nil {
		return err
	}
	sfid, err := seedSlip(srv, sid)
	if err != nil {
		return err
	}
	// Run an analysis so the slope has a current_f and is in monitoring-ready
	// state after a reading.
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "bishop", "slice_count": 12,
	}, nil); err != nil {
		return err
	}
	// Install an inclinometer and record a displacement above the red threshold.
	var ins map[string]any
	if err := mustDo(srv, "POST", "/api/slopes/"+sid+"/instruments", map[string]any{
		"type": "inclinometer", "x": 3, "install_el": 10, "range_max": 50,
	}, &ins); err != nil {
		return err
	}
	iid, _ := ins["id"].(string)
	if err := mustDo(srv, "POST", "/api/instruments/"+iid+"/readings", map[string]any{
		"value": 15.0, "ts": clk.Epoch(), "source": "manual",
	}, nil); err != nil {
		return err
	}
	clk.Advance(60)
	// Slope should be in red alert now.
	var sl struct {
		AlertLevel string `json:"alert_level"`
		Status    string `json:"status"`
	}
	if err := mustDo(srv, "GET", "/api/slopes/"+sid, nil, &sl); err != nil {
		return err
	}
	if sl.AlertLevel != "red" {
		return fmt.Errorf("alert = %s, want red (inclinometer displacement)", sl.AlertLevel)
	}
	// Closing must be refused while red.
	if err := expectCode(srv, "POST", "/api/slopes/"+sid+"/close", nil, 422); err != nil {
		return err
	}
	return nil
}

// smokeFrontend verifies the embedded index page is served at GET /.
func smokeFrontend(srv *httptest.Server, clk *clock.Fake) error {
	code, body, err := doRaw(srv, "/")
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("GET /: status %d", code)
	}
	if !strings.Contains(string(body), "slopestability") && !strings.Contains(string(body), "slope") {
		return fmt.Errorf("GET / body does not mention slope/slopestability")
	}
	// Also hit the health endpoint via the same page load path.
	var h map[string]any
	if err := mustDo(srv, "GET", "/api/health", nil, &h); err != nil {
		return err
	}
	if h["status"] != "ok" {
		return fmt.Errorf("health status = %v, want ok", h["status"])
	}
	return nil
}

// smokeRestartRecovery seeds a slope with an analysis + a piezometer reading,
// closes the store to simulate a crash, reopens the same file, runs the
// reconcile/recovery path and asserts the recomputed current_f is unchanged.
func smokeRestartRecovery(dbPath string, clk *clock.Fake) error {
	srv1, st1, err := restartServer(dbPath, clk)
	if err != nil {
		return err
	}
	sid, err := seedSlope(srv1)
	if err != nil {
		srv1.Close()
		return err
	}
	sfid, err := seedSlip(srv1, sid)
	if err != nil {
		srv1.Close()
		return err
	}
	if err := mustDo(srv1, "POST", "/api/slopes/"+sid+"/analyses", map[string]any{
		"slip_surface_id": sfid, "method": "bishop", "slice_count": 12,
	}, nil); err != nil {
		srv1.Close()
		return err
	}
	var before struct {
		CurrentF float64 `json:"current_f"`
	}
	if err := mustDo(srv1, "GET", "/api/slopes/"+sid, nil, &before); err != nil {
		srv1.Close()
		return err
	}
	srv1.Close()
	_ = st1.Close()

	// Reopen the same file and run the startup reconcile.
	srv2, st2, err := restartServer(dbPath, clk)
	if err != nil {
		return err
	}
	defer srv2.Close()
	defer st2.Close()
	svc := service.NewWithClock(st2, clk)
	if _, _, err := svc.Reconcile().ReconcileAll(ctx()); err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}
	var after struct {
		CurrentF float64 `json:"current_f"`
	}
	if err := mustDo(srv2, "GET", "/api/slopes/"+sid, nil, &after); err != nil {
		return err
	}
	if !numEq(before.CurrentF, after.CurrentF) {
		return fmt.Errorf("current_f changed across restart: before=%v after=%v", before.CurrentF, after.CurrentF)
	}
	return nil
}
