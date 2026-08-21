package clock

import "testing"

func TestFakeClockAdvanceAndSet(t *testing.T) {
	fake := NewFake(100)
	fake.Advance(25)
	if got := fake.Epoch(); got != 125 {
		t.Fatalf("Epoch() = %d, want 125", got)
	}
	fake.Set(7)
	if got := fake.Epoch(); got != 7 {
		t.Fatalf("Epoch() after Set = %d, want 7", got)
	}
}
