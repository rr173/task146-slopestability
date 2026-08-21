package httpapi

import (
	"net/http"

	"task146-slopestability/internal/service"
)

// createSlope: POST /api/slopes
func (h *handlers) createSlope(w http.ResponseWriter, r *http.Request) {
	var in service.CreateSlopeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	sl, err := h.svc.CreateSlope(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sl)
}

func (h *handlers) listSlopes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListSlopes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handlers) getSlope(w http.ResponseWriter, r *http.Request) {
	sl, err := h.svc.GetSlope(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sl)
}

func (h *handlers) updateSlope(w http.ResponseWriter, r *http.Request) {
	var in service.UpdateSlopeGeometryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	sl, err := h.svc.UpdateSlopeGeometry(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sl)
}

func (h *handlers) deleteSlope(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteSlope(r.Context(), pathID(r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *handlers) closeSlope(w http.ResponseWriter, r *http.Request) {
	sl, err := h.svc.CloseSlope(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sl)
}

func (h *handlers) reconcile(w http.ResponseWriter, r *http.Request) {
	rc, dr, err := h.svc.Reconcile().ReconcileAll(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"recomputed": rc, "drifts": dr})
}

func (h *handlers) listEvents(w http.ResponseWriter, r *http.Request) {
	evs, err := h.svc.ListEvents(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, evs)
}

// --- Layers ---

func (h *handlers) addLayer(w http.ResponseWriter, r *http.Request) {
	var in service.LayerInput
	if !decodeJSON(w, r, &in) {
		return
	}
	l, err := h.svc.AddLayer(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (h *handlers) listLayers(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListLayers(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handlers) updateLayer(w http.ResponseWriter, r *http.Request) {
	var in service.LayerInput
	if !decodeJSON(w, r, &in) {
		return
	}
	l, err := h.svc.UpdateLayer(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (h *handlers) deleteLayer(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteLayer(r.Context(), pathID(r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
