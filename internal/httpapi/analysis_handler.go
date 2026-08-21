package httpapi

import (
	"net/http"

	"task146-slopestability/internal/service"
)

// --- Slip surfaces ---

func (h *handlers) createSlip(w http.ResponseWriter, r *http.Request) {
	var in service.SlipInput
	if !decodeJSON(w, r, &in) {
		return
	}
	sf, err := h.svc.CreateSlipSurface(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sf)
}

func (h *handlers) listSlip(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListSlipSurfaces(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// --- Analyses ---

func (h *handlers) submitAnalysis(w http.ResponseWriter, r *http.Request) {
	var in service.AnalysisInput
	if !decodeJSON(w, r, &in) {
		return
	}
	res, err := h.svc.SubmitAnalysis(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *handlers) listAnalyses(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListAnalyses(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handlers) getAnalysis(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.GetAnalysis(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// --- Critical search ---

func (h *handlers) searchCritical(w http.ResponseWriter, r *http.Request) {
	var in service.SearchCriticalInput
	if !decodeJSON(w, r, &in) {
		return
	}
	summary, slipID, err := h.svc.SearchCritical(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": summary, "critical_slip_id": slipID,
	})
}

func (h *handlers) getSearch(w http.ResponseWriter, r *http.Request) {
	sj, err := h.svc.GetCriticalSearch(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"summary_json": sj})
}
