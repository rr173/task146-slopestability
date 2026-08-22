package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"task146-slopestability/internal/geotech"
	"task146-slopestability/internal/model"
	"task146-slopestability/internal/store"
)

// --- Instrument ---

// InstrumentInput is the body for creating an instrument.
type InstrumentInput struct {
	Type      string  `json:"type"`
	X         float64 `json:"x"`
	InstallEl float64 `json:"install_el"`
	RangeMax  float64 `json:"range_max"`
}

// CreateInstrument stores a monitoring device.
func (s *Service) CreateInstrument(ctx context.Context, slopeID string, in InstrumentInput) (*model.Instrument, error) {
	t := model.InstrumentType(in.Type)
	if t != model.InstrumentPiezometer && t != model.InstrumentInclinometer {
		return nil, fmt.Errorf("%w: unknown instrument type", store.ErrInvariant)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ins := &model.Instrument{
		ID: nextID("ins"), SlopeID: slopeID, Type: t,
		X: in.X, InstallEl: in.InstallEl, RangeMax: in.RangeMax, CreatedAt: s.now(),
	}
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		if _, err := s.store.GetSlopeTx(ctx, tx, slopeID); err != nil {
			return err
		}
		if err := s.store.CreateInstrument(ctx, tx, ins); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: slopeID, Ts: s.now(), Kind: model.EvInstrumentAdd, Payload: marshalPayload(ins),
		})
	})
	if err != nil {
		return nil, err
	}
	return ins, nil
}

// ListInstruments returns the instruments for a slope.
func (s *Service) ListInstruments(ctx context.Context, slopeID string) ([]model.Instrument, error) {
	if _, err := s.store.GetSlope(ctx, slopeID); err != nil {
		return nil, err
	}
	return s.store.ListInstruments(ctx, slopeID)
}

// --- Reading + atomic recompute session ---

// ReadingInput is the body for recording a reading.
type ReadingInput struct {
	Value     float64 `json:"value"`
	Ts        int64   `json:"ts"`
	SessionID string  `json:"session_id"`
	Source    string  `json:"source"`
}

// AddReadingRecord stores a reading and atomically refreshes the derived
// monitoring state in the same transaction.
func (s *Service) AddReadingRecord(ctx context.Context, instrumentID string, in ReadingInput) (*model.Session, float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var session model.Session
	var recompF float64

	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		ins, err := s.store.GetInstrumentTx(ctx, tx, instrumentID)
		if err != nil {
			return err
		}
		slopeID := ins.SlopeID
		sl, err := s.store.GetSlopeTx(ctx, tx, slopeID)
		if err != nil {
			return err
		}
		if sl.Status == model.StatusClosed {
			return store.ErrStateConflict
		}

		// Resolve the session: reuse the provided one or create a new session.
		sesID := in.SessionID
		if sesID == "" {
			sesID = nextID("ses")
			session = model.Session{
				ID: sesID, SlopeID: slopeID, CreatedAt: in.Ts, Note: "auto from reading",
				AlertLevel: sl.AlertLevel, PrevAlert: sl.AlertLevel,
			}
			if err := s.store.CreateSession(ctx, tx, &session); err != nil {
				return err
			}
		} else {
			ex, err := s.store.GetSessionTx(ctx, tx, sesID)
			if err != nil {
				return err
			}
			session = *ex
			session.PrevAlert = ex.AlertLevel
		}

		rd := &model.Reading{
			ID: nextID("rdg"), InstrumentID: instrumentID, SlopeID: slopeID,
			Ts: in.Ts, Value: in.Value, SessionID: sesID,
			Source: model.ReadingSource(in.Source),
		}
		if rd.Source == "" {
			rd.Source = model.ReadingManual
		}
		if err := s.store.CreateReading(ctx, tx, rd); err != nil {
			return err
		}

		// A reading can refresh derived monitoring state when prior analysis data exists.
		layers, err := s.store.ListLayersTx(ctx, tx, slopeID)
		if err != nil {
			return err
		}
		analyses, err := s.store.ListAnalysesTx(ctx, tx, slopeID)
		if err != nil {
			return err
		}
		if len(analyses) == 0 || len(layers) == 0 {
			// No analysis to recompute against; just record the reading.
			return s.appendEvent(ctx, tx, &model.Event{
				SlopeID: slopeID, Ts: in.Ts, Kind: model.EvReadingAdd, Payload: marshalPayload(rd),
			})
		}
		last := analyses[0]
		sf, err := s.store.GetSlipSurfaceTx(ctx, tx, last.SlipSurfaceID)
		if err != nil {
			return err
		}
		waterTable := last.WaterTableEl
		if pr, err := s.store.LatestPiezometerReadingTx(ctx, tx, slopeID); err == nil {
			waterTable = pr.Value
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}

		// Promote to monitoring on first reading if still analyzing.
		newStatus := sl.Status
		if sl.Status != model.StatusMonitoring {
			newStatus = model.StatusMonitoring
		}

		prof := profileFromSlope(sl)
		// Replay the analysis-time surcharge so the recompute sees the same
		// construction load as the original run — otherwise a reading taken under
		// surcharge recomputes against an unladen model.
		prof.SurchargeQ = last.SurchargeQ
		if last.TensionCrackDepth > 0 {
			prof.TensionCrackEl = sl.CrestEl - last.TensionCrackDepth
		}
		reinf, _ := s.store.ListReinforcementsTx(ctx, tx, slopeID)
		gin := geotech.SolveInput{
			Profile: prof, Layers: layers, Cx: sf.Cx, Cz: sf.Cz, R: sf.Radius,
			N: last.SliceCount, WaterTableEl: waterTable, Kh: last.Kh, Kv: last.Kv,
			Reinforcements: reinforcementInputs(reinf, layers, sl),
		}
		res, serr := solveByMethod(last.Method, gin)
		f := res.F
		if serr != nil {
			f = 0
		}
		recompF = f

		// Alert level: base on F, then layer red triggers for instruments.
		alert := alertFromF(f)
		alert = s.applyInstrumentRedTriggers(ctx, tx, slopeID, alert, in.Ts)

		// Write the recomputed session result.
		if err := s.store.UpdateSessionResult(ctx, tx, session.ID, f, alert, true); err != nil {
			return err
		}
		session.RecomputedF = f
		session.AlertLevel = alert
		session.Reconciled = true

		// Update slope derived figures + status.
		if err := s.store.UpdateSlopeDerived(ctx, tx, slopeID, f, alert); err != nil {
			return err
		}
		if newStatus != sl.Status {
			if err := s.store.UpdateSlopeStatus(ctx, tx, slopeID, newStatus); err != nil {
				return err
			}
		}
		// Rewrite compliance rows for this slope from the recomputed F.
		if err := s.writeComplianceTx(ctx, tx, slopeID, session.ID, f); err != nil {
			return err
		}
		// Alert transition event if the level changed.
		if alert != session.PrevAlert {
			if err := s.appendEvent(ctx, tx, &model.Event{
				SlopeID: slopeID, Ts: in.Ts, Kind: model.EvAlertChange,
				Payload: marshalPayload(map[string]string{"from": string(session.PrevAlert), "to": string(alert)}),
			}); err != nil {
				return err
			}
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: slopeID, Ts: in.Ts, Kind: model.EvReadingAdd, Payload: marshalPayload(rd),
		})
	})
	if err != nil {
		return nil, 0, err
	}
	return &session, recompF, nil
}

// ListReadings returns readings for a slope, optionally filtered by instrument.
func (s *Service) ListReadings(ctx context.Context, slopeID, instrumentID string) ([]model.Reading, error) {
	if _, err := s.store.GetSlope(ctx, slopeID); err != nil {
		return nil, err
	}
	return s.store.ListReadings(ctx, slopeID, instrumentID)
}

// --- Session ---

// CreateSessionInput is the body for creating a monitoring session.
type CreateSessionInput struct {
	Note string `json:"note"`
}

// CreateSession establishes a monitoring session (does not recompute).
func (s *Service) CreateSession(ctx context.Context, slopeID string, in CreateSessionInput) (*model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ses *model.Session
	err := s.store.InTx(ctx, func(tx *sql.Tx) error {
		sl, err := s.store.GetSlopeTx(ctx, tx, slopeID)
		if err != nil {
			return err
		}
		if sl.Status == model.StatusClosed {
			return store.ErrStateConflict
		}
		ses = &model.Session{
			ID: nextID("ses"), SlopeID: slopeID, CreatedAt: s.now(), Note: in.Note,
			AlertLevel: sl.AlertLevel, PrevAlert: sl.AlertLevel,
		}
		if err := s.store.CreateSession(ctx, tx, ses); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, &model.Event{
			SlopeID: slopeID, Ts: s.now(), Kind: model.EvSessionCreate, Payload: marshalPayload(ses),
		})
	})
	if err != nil {
		return nil, err
	}
	return ses, nil
}

// GetSession returns one session.
func (s *Service) GetSession(ctx context.Context, id string) (*model.Session, error) {
	return s.store.GetSession(ctx, id)
}

// ListSessions returns the sessions for a slope.
func (s *Service) ListSessions(ctx context.Context, slopeID string) ([]model.Session, error) {
	if _, err := s.store.GetSlope(ctx, slopeID); err != nil {
		return nil, err
	}
	return s.store.ListSessions(ctx, slopeID)
}

// applyInstrumentRedTriggers layers on red-alert triggers that are independent
// of the recomputed F: excessive inclinometer displacement and a rapid
// piezometric rise within a 6h window.
func (s *Service) applyInstrumentRedTriggers(ctx context.Context, tx *sql.Tx, slopeID string, base model.AlertLevel, nowTs int64) model.AlertLevel {
	// Inclinometer: latest reading above the displacement threshold => red.
	if ir, err := s.store.LatestInclinometerReadingTx(ctx, tx, slopeID); err == nil {
		if math.Abs(ir.Value) >= InclinometerRedDispMM {
			return model.AlertRed
		}
	}
	// Piezometric surge: the latest piezometer reading rose more than the
	// delta threshold relative to the previous reading, within the window.
	if pzs, err := s.store.ListPiezometerReadingsTx(ctx, tx, slopeID); err == nil && len(pzs) >= 2 {
		last := pzs[len(pzs)-1]
		prev := pzs[len(pzs)-2]
		if last.Ts-prev.Ts <= PorePressureWindowS && last.Value-prev.Value >= PorePressureRedDelta {
			return model.AlertRed
		}
	}
	return base
}
