package core

import (
	"math"
	"testing"
)

func TestAddSafe(t *testing.T) {
	if v, err := AddSafe(10, 5); err != nil || v != 15 {
		t.Fatalf("got %v %v", v, err)
	}
	if _, err := AddSafe(math.MaxInt64, 1); err == nil {
		t.Fatalf("expected overflow")
	}
}

func TestNormalizeUserID(t *testing.T) {
	id, err := NormalizeUserID(" Alice ")
	if err != nil || id != "alice" {
		t.Fatalf("got %v %v", id, err)
	}
	if _, err := NormalizeUserID("   "); err == nil {
		t.Fatalf("expected empty error")
	}
}

func TestDefaultLevel(t *testing.T) {
	if DefaultLevel(0) != 1 {
		t.Fatal("min level should be 1")
	}
	if DefaultLevel(10_000) < 1 {
		t.Fatal("unexpected level")
	}
}

func TestValidateBadgeID(t *testing.T) {
	if err := ValidateBadgeID("onboarded_1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := ValidateBadgeID("bad badge"); err == nil {
		t.Fatalf("expected invalid badge err")
	}
}
