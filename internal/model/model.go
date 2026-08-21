// Package model defines the domain entities of the slope-stability engine.
// All entities carry JSON tags so the API and store layers share one wire
// shape; numeric magnitudes use float64 (physical units documented per field).
//
// Units (SI-ish geotechnical convention):
//   - elevations / depths / coordinates: metres (m)
//   - gamma (unit weight): kN/m³   - c (cohesion): kPa   - phi: degrees
//   - force (reinforcement, surcharge resultant): kN
//   - surcharge q: kN/m²  - angles: degrees
//   - safety factor F: dimensionless
//   - gamma_w (water unit weight) is 9.81 kN/m³ in the solver.
package model

// SlopeStatus is the slope lifecycle state.
type SlopeStatus string

const (
	StatusInvestigating SlopeStatus = "investigating"
	StatusDesigned      SlopeStatus = "designed"
	StatusAnalyzing     SlopeStatus = "analyzing"
	StatusMonitoring    SlopeStatus = "monitoring"
	StatusClosed        SlopeStatus = "closed"
)

// AlertLevel is the monitoring alert level. Upgrades are monotone
// green→yellow→red; downgrades require a recomputing session.
type AlertLevel string

const (
	AlertGreen  AlertLevel = "green"
	AlertYellow AlertLevel = "yellow"
	AlertRed    AlertLevel = "red"
)

// AnalysisMethod selects the limit-equilibrium formulation.
type AnalysisMethod string

const (
	MethodBishop    AnalysisMethod = "bishop"
	MethodFellenius AnalysisMethod = "fellenius"
	MethodJanbu     AnalysisMethod = "janbu"
)

// AnalysisStatus is the run lifecycle of a single analysis.
type AnalysisStatus string

const (
	AnalysisDraft     AnalysisStatus = "draft"
	AnalysisRunning   AnalysisStatus = "running"
	AnalysisConverged AnalysisStatus = "converged"
	AnalysisFailed    AnalysisStatus = "failed"
)

// ComplianceScenario selects the required-F threshold curve.
type ComplianceScenario string

const (
	ScenarioStatic    ComplianceScenario = "static"
	ScenarioTransient ComplianceScenario = "transient"
	ScenarioSeismic   ComplianceScenario = "seismic"
)

// SlipType selects the slip-surface geometry.
type SlipType string

const (
	SlipCircular SlipType = "circular"
	SlipNonCircular SlipType = "noncircular"
)

// ReinforcementType selects the reinforcement family.
type ReinforcementType string

const (
	ReinfGeotextile ReinforcementType = "geotextile"
	ReinfAnchor     ReinforcementType = "anchor"
)

// InstrumentType selects the monitoring instrument.
type InstrumentType string

const (
	InstrumentPiezometer   InstrumentType = "piezometer"
	InstrumentInclinometer InstrumentType = "inclinometer"
)

// ReadingSource is where a reading came from.
type ReadingSource string

const (
	ReadingManual ReadingSource = "manual"
	ReadingAuto   ReadingSource = "auto"
)

// Slope is the top-level entity: a single slope cross-section under analysis.
type Slope struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	CrestEl       float64     `json:"crest_el"`        // m, top of slope
	ToeEl         float64     `json:"toe_el"`          // m, toe of slope
	Height        float64     `json:"height"`          // m, crest-toe (derived but stored for speed)
	SlopeAngle    float64     `json:"slope_angle"`     // degrees from horizontal
	WaterTableEl  float64     `json:"water_table_el"`  // m; 0/absent => dry (use layer-weighted)
	SurchargeX    float64     `json:"surcharge_x"`     // m, x where surcharge starts (toe side origin)
	SurchargeQ    float64     `json:"surcharge_q"`     // kN/m²
	TensionCrackEl float64    `json:"tension_crack_el"` // m; depth expressed as elevation of crack bottom
	Status        SlopeStatus `json:"status"`
	CurrentF      float64     `json:"current_f"`       // latest recomputed safety factor (0 => none)
	AlertLevel    AlertLevel  `json:"alert_level"`
	CreatedAt     int64       `json:"created_at"`      // epoch seconds
}

// SoilLayer is one stratigraphic layer of a slope.
type SoilLayer struct {
	ID      string  `json:"id"`
	SlopeID string  `json:"slope_id"`
	Name    string  `json:"name"`
	C       float64 `json:"c"`        // kPa cohesion
	Phi     float64 `json:"phi"`     // degrees internal friction
	Gamma   float64 `json:"gamma"`   // kN/m³ unit weight (total)
	TopEl   float64 `json:"top_el"`  // m, top elevation of the layer
	BotEl   float64 `json:"bot_el"`  // m, bottom elevation of the layer
	Order   int     `json:"order"`   // sort order (top layer first)
}

// SlipSurface is a trial or critical slip surface.
type SlipSurface struct {
	ID         string   `json:"id"`
	SlopeID    string   `json:"slope_id"`
	Type       SlipType `json:"type"`
	Cx         float64  `json:"cx"`          // m, circle centre x (circular)
	Cz         float64  `json:"cz"`          // m, circle centre z (circular)
	Radius     float64  `json:"radius"`      // m (circular)
	Polyline   string   `json:"polyline"`    // JSON points (noncircular); "" for circular
	IsCritical bool     `json:"is_critical"` // true if chosen by critical search
	DerivedF   float64  `json:"derived_f"`   // F computed on this surface (0 => none)
	CreatedAt  int64    `json:"created_at"`
}

// Analysis is a single limit-equilibrium run.
type Analysis struct {
	ID                string         `json:"id"`
	SlopeID           string         `json:"slope_id"`
	SlipSurfaceID     string         `json:"slip_surface_id"`
	Method            AnalysisMethod `json:"method"`
	SliceCount        int            `json:"slice_count"`
	Kh                float64        `json:"kh"`                 // horizontal seismic coefficient
	Kv                float64        `json:"kv"`                 // vertical seismic coefficient
	SurchargeQ        float64         `json:"surcharge_q"`        // kN/m² (overrides slope's at run time)
	WaterTableEl      float64         `json:"water_table_el"`    // m used for this run
	TensionCrackDepth float64        `json:"tension_crack_depth"` // m below crest
	Iterations        int            `json:"iterations"`
	FinalF            float64        `json:"final_f"`            // 0/NaN => failed/unset
	Status            AnalysisStatus `json:"status"`
	CreatedAt         int64          `json:"created_at"`
}

// Reinforcement is a stabilising element.
type Reinforcement struct {
	ID         string            `json:"id"`
	SlopeID    string            `json:"slope_id"`
	Type       ReinforcementType `json:"type"`
	// Geotextile parameters.
	TensileKNI float64 `json:"tensile_kni"` // kN/m design tensile force
	EmbedTopEl float64 `json:"embed_top_el"`
	EmbedBotEl float64 `json:"embed_bot_el"`
	// Anchor parameters.
	CapacityKN float64 `json:"capacity_kn"` // kN design capacity
	AngleDeg   float64 `json:"angle_deg"`   // degrees from horizontal, down into slope
	DepthEl    float64 `json:"depth_el"`    // m, anchor bond elevation
	// Computed (recomputable) effective force taken into the resisting moment.
	ComputedEff float64 `json:"computed_eff"` // kN, min(material, geotechnical) per element
	CreatedAt   int64  `json:"created_at"`
}

// Instrument is a monitoring device installed on a slope.
type Instrument struct {
	ID         string          `json:"id"`
	SlopeID    string          `json:"slope_id"`
	Type       InstrumentType  `json:"type"`
	X          float64         `json:"x"`         // m, cross-section coordinate
	InstallEl  float64         `json:"install_el"` // m
	RangeMax   float64         `json:"range_max"`
	CreatedAt  int64           `json:"created_at"`
}

// Reading is one measurement from an instrument at a point in time.
type Reading struct {
	ID           string `json:"id"`
	InstrumentID string `json:"instrument_id"`
	SlopeID      string `json:"slope_id"` // denormalised for fast event replay
	Ts           int64  `json:"ts"`      // epoch seconds
	Value        float64 `json:"value"`  // piezometer: head (m); inclinometer: disp (mm)
	SessionID    string `json:"session_id"`
	Source       ReadingSource `json:"source"`
}

// Session is a monitoring session: a recomputation checkpoint.
type Session struct {
	ID           string     `json:"id"`
	SlopeID      string     `json:"slope_id"`
	CreatedAt    int64      `json:"created_at"`
	Note         string     `json:"note"`
	RecomputedF  float64    `json:"recomputed_f"` // F recomputed from latest readings
	AlertLevel   AlertLevel `json:"alert_level"`
	Reconciled   bool       `json:"reconciled"`
	PrevAlert    AlertLevel `json:"prev_alert"` // alert before this session
}

// Compliance is a pass/fail verdict against a required-F curve.
type Compliance struct {
	ID         string            `json:"id"`
	SlopeID    string            `json:"slope_id"`
	SessionID  string            `json:"session_id"`
	Scenario   ComplianceScenario `json:"scenario"`
	RequiredF  float64           `json:"required_f"`
	ActualF    float64           `json:"actual_f"`
	Verdict    string            `json:"verdict"` // pass / fail
	Detail     string            `json:"detail"`
}

// EventKind enumerates the event-stream kinds persisted for replay.
type EventKind string

const (
	EvSlopeCreate     EventKind = "slope_create"
	EvSlopeUpdate     EventKind = "slope_update"
	EvSlopeStateChange EventKind = "slope_state_change"
	EvLayerAdd        EventKind = "layer_add"
	EvLayerUpdate     EventKind = "layer_update"
	EvLayerDelete     EventKind = "layer_delete"
	EvSlipAdd         EventKind = "slip_add"
	EvSlipCritical    EventKind = "slip_critical"
	EvAnalysisRun     EventKind = "analysis_run"
	EvReinforceAdd    EventKind = "reinforce_add"
	EvInstrumentAdd   EventKind = "instrument_add"
	EvReadingAdd      EventKind = "reading_add"
	EvSessionCreate   EventKind = "session_create"
	EvAlertChange     EventKind = "alert_change"
)

// Event is an append-only event-stream row; the authoritative replay source.
type Event struct {
	ID       int64     `json:"id"`
	SlopeID  string    `json:"slope_id"`
	Ts       int64     `json:"ts"`
	Kind     EventKind `json:"kind"`
	Payload  string    `json:"payload"` // JSON
}

// SliceResult is one slice's contribution in an analysis (derived detail).
type SliceResult struct {
	Number   int     `json:"number"`
	XM       float64 `json:"x_m"`        // m, slice centre x
	ZG       float64 `json:"z_g"`        // m, ground elevation at centre
	ZS       float64 `json:"z_s"`        // m, slip elevation at centre
	AlphaDeg float64 `json:"alpha_deg"`  // degrees
	WidthB   float64 `json:"width_b"`    // m
	Height   float64 `json:"height"`     // m
	WeightW  float64 `json:"weight_w"`   // kN/m
	PoreU    float64 `json:"pore_u"`     // kPa at base
	Cohesion float64 `json:"cohesion"`   // kPa (layer at base)
	Phi      float64 `json:"phi"`        // degrees (layer at base)
	Resist   float64 `json:"resist"`     // kN·m/m resisting moment contribution / R
	Drive    float64 `json:"drive"`      // kN·m/m driving moment contribution / R
}

// SearchSummary records a critical-surface grid search.
type SearchSummary struct {
	Total     int     `json:"total"`      // grid points enumerated
	Evaluated int     `json:"evaluated"`   // legal & solved
	Skipped   int     `json:"skipped"`     // illegal geometry
	MinF      float64 `json:"min_f"`
	BestCx    float64 `json:"best_cx"`
	BestCz    float64 `json:"best_cz"`
	BestR     float64 `json:"best_r"`
	Converged bool    `json:"converged"`
}
