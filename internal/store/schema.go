package store

// schema is the authoritative DDL. INTEGER epoch seconds store all timestamps;
// REAL stores all physical magnitudes. Foreign keys enforce child rows
// reference existing slopes. The events table is the replay source; derived
// figures (analyses.final_f, slopes.current_f, sessions.recomputed_f) are all
// recomputable from the authoritative inputs and the events stream.
const schema = `
CREATE TABLE IF NOT EXISTS slopes (
	id               TEXT PRIMARY KEY,
	name             TEXT NOT NULL,
	crest_el         REAL NOT NULL,
	toe_el           REAL NOT NULL,
	height           REAL NOT NULL,
	slope_angle      REAL NOT NULL,
	water_table_el   REAL NOT NULL DEFAULT 0,
	surcharge_x      REAL NOT NULL DEFAULT 0,
	surcharge_q      REAL NOT NULL DEFAULT 0,
	tension_crack_el REAL NOT NULL DEFAULT 0,
	status           TEXT NOT NULL DEFAULT 'investigating',
	current_f        REAL NOT NULL DEFAULT 0,
	alert_level      TEXT NOT NULL DEFAULT 'green',
	created_at       INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS soil_layers (
	id      TEXT PRIMARY KEY,
	slope_id TEXT NOT NULL REFERENCES slopes(id),
	name    TEXT NOT NULL,
	c       REAL NOT NULL,
	phi     REAL NOT NULL,
	gamma   REAL NOT NULL,
	top_el  REAL NOT NULL,
	bot_el  REAL NOT NULL,
	ord     INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_layers_slope ON soil_layers(slope_id, ord);

CREATE TABLE IF NOT EXISTS slip_surfaces (
	id          TEXT PRIMARY KEY,
	slope_id    TEXT NOT NULL REFERENCES slopes(id),
	type        TEXT NOT NULL,
	cx          REAL NOT NULL DEFAULT 0,
	cz          REAL NOT NULL DEFAULT 0,
	radius      REAL NOT NULL DEFAULT 0,
	polyline    TEXT NOT NULL DEFAULT '',
	is_critical INTEGER NOT NULL DEFAULT 0,
	derived_f   REAL NOT NULL DEFAULT 0,
	created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_slip_slope ON slip_surfaces(slope_id);

CREATE TABLE IF NOT EXISTS analyses (
	id                  TEXT PRIMARY KEY,
	slope_id            TEXT NOT NULL REFERENCES slopes(id),
	slip_surface_id     TEXT NOT NULL REFERENCES slip_surfaces(id),
	method              TEXT NOT NULL,
	slice_count         INTEGER NOT NULL,
	kh                  REAL NOT NULL,
	kv                  REAL NOT NULL,
	surcharge_q         REAL NOT NULL,
	water_table_el      REAL NOT NULL,
	tension_crack_depth REAL NOT NULL,
	iterations          INTEGER NOT NULL DEFAULT 0,
	final_f             REAL NOT NULL DEFAULT 0,
	status              TEXT NOT NULL DEFAULT 'draft',
	slices_json         TEXT NOT NULL DEFAULT '',
	created_at          INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_analyses_slope ON analyses(slope_id, created_at);

CREATE TABLE IF NOT EXISTS critical_searches (
	id          TEXT PRIMARY KEY,
	slope_id    TEXT NOT NULL REFERENCES slopes(id),
	summary_json TEXT NOT NULL DEFAULT '',
	created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_searches_slope ON critical_searches(slope_id, created_at);

CREATE TABLE IF NOT EXISTS reconcile_log (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	slope_id   TEXT NOT NULL,
	run_at     INTEGER NOT NULL,
	recomputed INTEGER NOT NULL DEFAULT 0,
	drifts     INTEGER NOT NULL DEFAULT 0,
	note       TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_recon_slope ON reconcile_log(slope_id, run_at);

CREATE TABLE IF NOT EXISTS reinforcements (
	id           TEXT PRIMARY KEY,
	slope_id     TEXT NOT NULL REFERENCES slopes(id),
	type         TEXT NOT NULL,
	tensile_kni  REAL NOT NULL DEFAULT 0,
	embed_top_el REAL NOT NULL DEFAULT 0,
	embed_bot_el REAL NOT NULL DEFAULT 0,
	capacity_kn  REAL NOT NULL DEFAULT 0,
	angle_deg    REAL NOT NULL DEFAULT 0,
	depth_el     REAL NOT NULL DEFAULT 0,
	computed_eff REAL NOT NULL DEFAULT 0,
	created_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reinf_slope ON reinforcements(slope_id);

CREATE TABLE IF NOT EXISTS instruments (
	id          TEXT PRIMARY KEY,
	slope_id    TEXT NOT NULL REFERENCES slopes(id),
	type        TEXT NOT NULL,
	x           REAL NOT NULL,
	install_el  REAL NOT NULL,
	range_max   REAL NOT NULL DEFAULT 0,
	created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_instruments_slope ON instruments(slope_id);

CREATE TABLE IF NOT EXISTS readings (
	id            TEXT PRIMARY KEY,
	instrument_id TEXT NOT NULL REFERENCES instruments(id),
	slope_id      TEXT NOT NULL,
	ts            INTEGER NOT NULL,
	value         REAL NOT NULL,
	session_id    TEXT NOT NULL DEFAULT '',
	source        TEXT NOT NULL DEFAULT 'manual'
);

CREATE INDEX IF NOT EXISTS idx_readings_instrument ON readings(instrument_id, ts);
CREATE INDEX IF NOT EXISTS idx_readings_slope_ts ON readings(slope_id, ts);

CREATE TABLE IF NOT EXISTS sessions (
	id            TEXT PRIMARY KEY,
	slope_id      TEXT NOT NULL REFERENCES slopes(id),
	created_at    INTEGER NOT NULL,
	note          TEXT NOT NULL DEFAULT '',
	recomputed_f  REAL NOT NULL DEFAULT 0,
	alert_level   TEXT NOT NULL DEFAULT 'green',
	reconciled    INTEGER NOT NULL DEFAULT 0,
	prev_alert    TEXT NOT NULL DEFAULT 'green'
);

CREATE INDEX IF NOT EXISTS idx_sessions_slope ON sessions(slope_id, created_at);

CREATE TABLE IF NOT EXISTS compliances (
	id          TEXT PRIMARY KEY,
	slope_id    TEXT NOT NULL REFERENCES slopes(id),
	session_id  TEXT NOT NULL DEFAULT '',
	scenario    TEXT NOT NULL,
	required_f  REAL NOT NULL,
	actual_f    REAL NOT NULL,
	verdict     TEXT NOT NULL,
	detail      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_compliance_slope ON compliances(slope_id, scenario);

CREATE TABLE IF NOT EXISTS events (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	slope_id TEXT NOT NULL,
	ts      INTEGER NOT NULL,
	kind    TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_events_slope_ts ON events(slope_id, ts);

CREATE TABLE IF NOT EXISTS schema_meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`
