package idlib

import (
	"strings"
	"testing"
)

func TestNewTagsAndUniquelyAdvancesIDs(t *testing.T) {
	first := New("slp")
	second := New("slp")
	if !strings.HasPrefix(first, "slp_") || !strings.HasPrefix(second, "slp_") {
		t.Fatalf("IDs must retain prefix: %q, %q", first, second)
	}
	if first == second {
		t.Fatalf("consecutive IDs must be distinct: %q", first)
	}
}
