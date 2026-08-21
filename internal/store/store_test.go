package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"task146-slopestability/internal/model"
)

func TestSlopeRoundTripInsideTransaction(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "slope.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	want := &model.Slope{ID: "s1", Name: "east-cut", CrestEl: 20, ToeEl: 0, Height: 20, SlopeAngle: 45, Status: model.StatusInvestigating, AlertLevel: model.AlertGreen, CreatedAt: 1}
	if err := st.InTx(ctx, func(tx *sql.Tx) error { return st.CreateSlope(ctx, tx, want) }); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetSlope(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name || got.Height != want.Height || got.Status != want.Status {
		t.Fatalf("stored slope = %#v, want %#v", got, want)
	}
}
