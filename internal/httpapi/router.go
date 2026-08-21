// Package httpapi wires the slope-stability services to HTTP routes. It owns
// the mux, the request/response JSON shape and the error → status mapping. The
// self-check smoke test and the production binary share the same mux (via
// NewMux) so a single code path is exercised end-to-end.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"task146-slopestability/internal/service"
	"task146-slopestability/internal/store"
)

// Version is the API version, surfaced in the health check.
const Version = "v1.0.0"

// Services is the bundle of business services the handlers depend on.
type Services struct {
	Svc *service.Service
}

// NewMux builds the HTTP mux from services and an embedded frontend filesystem.
func NewMux(svc Services, frontend fs.FS) http.Handler {
	mux := http.NewServeMux()
	h := &handlers{svc: svc.Svc}

	// Slopes.
	mux.HandleFunc("POST /api/slopes", h.createSlope)
	mux.HandleFunc("GET /api/slopes", h.listSlopes)
	mux.HandleFunc("GET /api/slopes/{id}", h.getSlope)
	mux.HandleFunc("PATCH /api/slopes/{id}", h.updateSlope)
	mux.HandleFunc("DELETE /api/slopes/{id}", h.deleteSlope)
	mux.HandleFunc("POST /api/slopes/{id}/close", h.closeSlope)
	mux.HandleFunc("POST /api/slopes/{id}/reconcile", h.reconcile)
	mux.HandleFunc("GET /api/slopes/{id}/events", h.listEvents)

	// Layers.
	mux.HandleFunc("POST /api/slopes/{id}/layers", h.addLayer)
	mux.HandleFunc("GET /api/slopes/{id}/layers", h.listLayers)
	mux.HandleFunc("PATCH /api/layers/{id}", h.updateLayer)
	mux.HandleFunc("DELETE /api/layers/{id}", h.deleteLayer)

	// Slip surfaces.
	mux.HandleFunc("POST /api/slopes/{id}/slip-surfaces", h.createSlip)
	mux.HandleFunc("GET /api/slopes/{id}/slip-surfaces", h.listSlip)
	mux.HandleFunc("POST /api/slopes/{id}/search-critical", h.searchCritical)
	mux.HandleFunc("GET /api/searches/{id}", h.getSearch)

	// Analyses.
	mux.HandleFunc("POST /api/slopes/{id}/analyses", h.submitAnalysis)
	mux.HandleFunc("GET /api/slopes/{id}/analyses", h.listAnalyses)
	mux.HandleFunc("GET /api/analyses/{id}", h.getAnalysis)

	// Reinforcements.
	mux.HandleFunc("POST /api/slopes/{id}/reinforcements", h.addReinforcement)
	mux.HandleFunc("GET /api/slopes/{id}/reinforcements", h.listReinforcements)
	mux.HandleFunc("DELETE /api/reinforcements/{id}", h.deleteReinforcement)

	// Instruments & readings.
	mux.HandleFunc("POST /api/slopes/{id}/instruments", h.createInstrument)
	mux.HandleFunc("GET /api/slopes/{id}/instruments", h.listInstruments)
	mux.HandleFunc("POST /api/instruments/{id}/readings", h.addReading)
	mux.HandleFunc("GET /api/slopes/{id}/readings", h.listReadings)

	// Sessions & compliance.
	mux.HandleFunc("POST /api/slopes/{id}/sessions", h.createSession)
	mux.HandleFunc("GET /api/slopes/{id}/sessions", h.listSessions)
	mux.HandleFunc("GET /api/sessions/{id}", h.getSession)
	mux.HandleFunc("GET /api/slopes/{id}/compliance", h.getCompliance)

	// Health.
	mux.HandleFunc("GET /api/health", h.health)

	// Frontend.
	if frontend != nil {
		mux.Handle("GET /", http.FileServer(http.FS(frontend)))
	}
	return mux
}

// handlers holds the service reference for the routes.
type handlers struct {
	svc *service.Service
}

// health is the liveness probe.
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
}

// writeJSON serializes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError maps a domain error to an HTTP status and writes a JSON body.
func writeError(w http.ResponseWriter, err error) {
	status := errorStatus(err)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// decodeJSON reads a JSON body into v. Returns 400 on a decode error. An empty
// body (EOF) is allowed and leaves v untouched.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil {
		return true
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

// errorStatus maps a store/service error to an HTTP status code.
func errorStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, store.ErrStateConflict), errors.Is(err, store.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, store.ErrInvariant),
		errors.Is(err, store.ErrGeometry),
		errors.Is(err, store.ErrAlertBlocked),
		errors.Is(err, store.ErrNotConverged),
		errors.Is(err, store.ErrNotReconciled),
		errors.Is(err, store.ErrReadingStale):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// pathID extracts the {id} path variable from the request.
func pathID(r *http.Request) string { return r.PathValue("id") }

// authBearer is unused but kept for parity with sibling tasks.
func authBearer(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}
