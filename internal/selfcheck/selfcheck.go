// Package selfcheck runs the --smoke-test for the slope-stability engine.
// Each scenario builds its own temporary SQLite file and httptest server so
// contract assertions don't trip over state left by an earlier scenario. The
// restart scenario seeds state, closes the store, reopens the same file and
// asserts the recomputed F is unchanged.
//
// The smoke test never sleeps and never touches the network; it talks to the
// real mux over httptest.NewServer so the HTTP/JSON contract itself is
// exercised end to end, including the embedded frontend route.
package selfcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"task146-slopestability/internal/clock"
	"task146-slopestability/internal/httpapi"
	"task146-slopestability/internal/service"
	"task146-slopestability/internal/store"
	"task146-slopestability/internal/webfs"
)

// Run executes every smoke scenario. Returns the first failure.
func Run() error {
	dir, err := os.MkdirTemp("", "slopestab-smoke-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	clk := clock.NewFake(parseTime("2026-01-15T09:00:00Z"))

	cases := []struct {
		name string
		fn   func(srv *httptest.Server, clk *clock.Fake) error
	}{
		{"full-flow-analysis", smokeFullFlow},
		{"bishop-vs-fellenius", smokeBishopVsFellenius},
		{"max-iter-failure", smokeMaxIterFailure},
		{"critical-search", smokeCriticalSearch},
		{"reinforcement-weakest-link", smokeReinforcementWeakest},
		{"pore-pressure-priority", smokePorePressurePriority},
		{"alert-state-machine", smokeAlertStateMachine},
		{"frontend-page-served", smokeFrontend},
	}
	for i, c := range cases {
		dbPath := filepath.Join(dir, fmt.Sprintf("smoke-%02d.db", i))
		srv, err := newServer(dbPath, clk)
		if err != nil {
			return fmt.Errorf("%s: new server: %w", c.name, err)
		}
		if err := c.fn(srv, clk); err != nil {
			srv.Close()
			return fmt.Errorf("%s: %w", c.name, err)
		}
		srv.Close()
	}

	if err := smokeRestartRecovery(filepath.Join(dir, "smoke-recover.db"), clk); err != nil {
		return fmt.Errorf("restart-recovery: %w", err)
	}
	return nil
}

// newServer opens a fresh SQLite file, builds the service (with the injected
// clock) + mux and returns an httptest server over the real HTTP handler tree.
func newServer(dbPath string, clk clock.Clock) (*httptest.Server, error) {
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	svc := service.NewWithClock(st, clk)
	webFS := webfs.FS()
	mux := httpapi.NewMux(httpapi.Services{Svc: svc}, webFS)
	srv := httptest.NewServer(mux)
	srv.Config.RegisterOnShutdown(func() { _ = st.Close() })
	return srv, nil
}

// restartServer opens a fresh store+service over an existing dbPath and returns
// a test server, plus the underlying store so the scenario can close it to
// simulate a crash.
func restartServer(dbPath string, clk clock.Clock) (*httptest.Server, *store.Store, error) {
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}
	svc := service.NewWithClock(st, clk)
	webFS := webfs.FS()
	mux := httpapi.NewMux(httpapi.Services{Svc: svc}, webFS)
	srv := httptest.NewServer(mux)
	return srv, st, nil
}

// --- HTTP helpers ---

func doJSON(srv *httptest.Server, method, path string, body any) (int, []byte, error) {
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, path, nil)
	} else {
		buf, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(buf))
		r.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(rec, r)
	return rec.Code, rec.Body.Bytes(), nil
}

// mustDo performs an HTTP call and returns the decoded body; it fails the
// scenario immediately on a non-2xx status (success-expecting helpers must
// surface non-200s, not swallow them).
func mustDo(srv *httptest.Server, method, path string, body any, out any) error {
	code, respBody, err := doJSON(srv, method, path, body)
	if err != nil {
		return err
	}
	if code < 200 || code >= 300 {
		return fmt.Errorf("%s %s: status %d: %s", method, path, code, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		if e := json.Unmarshal(respBody, out); e != nil {
			return fmt.Errorf("%s %s: decode: %w", method, path, e)
		}
	}
	return nil
}

// expectCode asserts the HTTP status without decoding the body.
func expectCode(srv *httptest.Server, method, path string, body any, wantCode int) error {
	code, respBody, err := doJSON(srv, method, path, body)
	if err != nil {
		return err
	}
	if code != wantCode {
		return fmt.Errorf("%s %s: want status %d, got %d: %s", method, path, wantCode, code, string(respBody))
	}
	return nil
}

// doRaw performs a GET and returns the raw body bytes (for the frontend page).
func doRaw(srv *httptest.Server, path string) (int, []byte, error) {
	r := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(rec, r)
	return rec.Code, rec.Body.Bytes(), nil
}

func ctx() context.Context { return context.Background() }

func parseTime(s string) int64 {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0
	}
	return t.Unix()
}


// seedSlope builds a standard slope with two layers and returns the slope id.
func seedSlope(srv *httptest.Server) (string, error) {
	var sl struct {
		ID string `json:"id"`
	}
	if err := mustDo(srv, "POST", "/api/slopes", map[string]any{
		"name": "smoke-slope", "crest_el": 20.0, "toe_el": 0.0,
		"slope_angle": 45.0, "water_table_el": 0,
	}, &sl); err != nil {
		return "", err
	}
	// Layer 1: clay, c=15, phi=20, gamma=18, elev 20..10
	if err := mustDo(srv, "POST", "/api/slopes/"+sl.ID+"/layers", map[string]any{
		"name": "clay", "c": 15, "phi": 20, "gamma": 18, "top_el": 20, "bot_el": 10, "order": 1,
	}, nil); err != nil {
		return "", err
	}
	// Layer 2: dense sand, c=5, phi=32, gamma=19, elev 10..-5
	if err := mustDo(srv, "POST", "/api/slopes/"+sl.ID+"/layers", map[string]any{
		"name": "sand", "c": 5, "phi": 32, "gamma": 19, "top_el": 10, "bot_el": -5, "order": 2,
	}, nil); err != nil {
		return "", err
	}
	return sl.ID, nil
}

// seedSlip adds a circular slip surface and returns its id.
func seedSlip(srv *httptest.Server, slopeID string) (string, error) {
	var sf struct {
		ID string `json:"id"`
	}
	if err := mustDo(srv, "POST", "/api/slopes/"+slopeID+"/slip-surfaces", map[string]any{
		"type": "circular", "cx": 5, "cz": 30, "radius": 25,
	}, &sf); err != nil {
		return "", err
	}
	return sf.ID, nil
}

// numEq reports whether two floats are equal within 1e-3 (smoke tolerance).
func numEq(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-3
}
