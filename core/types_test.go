package core

import "testing"

func TestAddSafe(t *testing.T) {
	if v, err := AddSafe(10, 5); err != nil || v != 15 {
		t.Fatalf("got %v %v", v, err)
	}
}

func TestNormalizeUserID(t *testing.T) {
	id, err := NormalizeUserID(" Alice ")
	if err != nil || id != "alice" {
		t.Fatalf("got %v %v", id, err)
	}
}

func TestDefaultLevel(t *testing.T) {
	if DefaultLevel(0) != 1 {
		t.Fatal("min level should be 1")
	}
}
