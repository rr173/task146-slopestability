package httpapi

import (
	"net/http"

	"task146-slopestability/internal/service"
)

// --- Reinforcements ---

func (h *handlers) addReinforcement(w http.ResponseWriter, r *http.Request) {
	var in service.ReinforcementInput
	if !decodeJSON(w, r, &in) {
		return
	}
	rn, err := h.svc.AddReinforcement(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rn)
}

func (h *handlers) listReinforcements(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListReinforcements(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handlers) deleteReinforcement(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteReinforcement(r.Context(), pathID(r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Instruments ---

func (h *handlers) createInstrument(w http.ResponseWriter, r *http.Request) {
	var in service.InstrumentInput
	if !decodeJSON(w, r, &in) {
		return
	}
	ins, err := h.svc.CreateInstrument(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ins)
}

func (h *handlers) listInstruments(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListInstruments(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handlers) addReading(w http.ResponseWriter, r *http.Request) {
	var in service.ReadingInput
	if !decodeJSON(w, r, &in) {
		return
	}
	ses, f, err := h.svc.AddReadingRecord(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": ses, "current_f": f})
}

func (h *handlers) listReadings(w http.ResponseWriter, r *http.Request) {
	instrumentID := r.URL.Query().Get("instrument")
	out, err := h.svc.ListReadings(r.Context(), pathID(r), instrumentID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// --- Sessions & compliance ---

func (h *handlers) createSession(w http.ResponseWriter, r *http.Request) {
	var in service.CreateSessionInput
	if !decodeJSON(w, r, &in) {
		return
	}
	ses, err := h.svc.CreateSession(r.Context(), pathID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ses)
}

func (h *handlers) listSessions(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListSessions(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handlers) getSession(w http.ResponseWriter, r *http.Request) {
	ses, err := h.svc.GetSession(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ses)
}

func (h *handlers) getCompliance(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetCompliance(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
