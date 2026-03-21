package adapters

import (
	"testing"
)

func TestAllReturnsAdapters(t *testing.T) {
	all := All()
	if len(all) != 2 {
		t.Fatalf("expected 2 adapters, got %d", len(all))
	}

	names := map[string]bool{}
	for _, a := range all {
		names[a.Name()] = true
	}
	if !names["claude-code"] {
		t.Error("missing claude-code adapter")
	}
	if !names["opencode"] {
		t.Error("missing opencode adapter")
	}
}

func TestGetFindsAdapter(t *testing.T) {
	a := Get("claude-code")
	if a == nil {
		t.Fatal("expected claude-code adapter")
	}
	if a.Name() != "claude-code" {
		t.Errorf("expected claude-code, got %s", a.Name())
	}
}

func TestGetReturnsNilForUnknown(t *testing.T) {
	a := Get("cursor")
	if a != nil {
		t.Error("expected nil for unknown adapter")
	}
}
