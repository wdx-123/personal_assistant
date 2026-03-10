package util

import (
	"testing"
	"time"
)

func TestApplyTTLJitterWithoutJitter(t *testing.T) {
	base := 5 * time.Minute
	got := ApplyTTLJitter(base, 0)
	if got != base {
		t.Fatalf("ApplyTTLJitter() = %v, want %v", got, base)
	}
}

func TestApplyTTLJitterWithJitter(t *testing.T) {
	base := 30 * time.Second
	maxJitter := 5 * time.Second

	for i := 0; i < 100; i++ {
		got := ApplyTTLJitter(base, maxJitter)
		if got < base || got > base+maxJitter {
			t.Fatalf("ApplyTTLJitter() = %v, want in [%v, %v]", got, base, base+maxJitter)
		}
	}
}

func TestApplyTTLJitterUsesDefaultBaseTTL(t *testing.T) {
	got := ApplyTTLJitter(0, 0)
	if got != 10*time.Minute {
		t.Fatalf("ApplyTTLJitter() = %v, want %v", got, 10*time.Minute)
	}
}
