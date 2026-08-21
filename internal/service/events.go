package service

import (
	"context"
	"database/sql"
	"encoding/json"

	"task146-slopestability/internal/model"
)

// ListEvents returns the event stream for a slope (the replay/audit source).
func (s *Service) ListEvents(ctx context.Context, slopeID string) ([]model.Event, error) {
	if _, err := s.store.GetSlope(ctx, slopeID); err != nil {
		return nil, err
	}
	return s.store.ListEvents(ctx, slopeID)
}

// appendEvent is the internal tx-scoped event appender used by every mutating
// method so the event stream stays the authoritative replay source.
func (s *Service) appendEvent(ctx context.Context, tx *sql.Tx, e *model.Event) error {
	return s.store.AppendEvent(ctx, tx, e)
}

// marshalPayload encodes a value to JSON for an event payload.
func marshalPayload(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// unmarshalPayload decodes a JSON payload into v.
func unmarshalPayload(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}
