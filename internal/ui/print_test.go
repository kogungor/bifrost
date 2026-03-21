package ui

import (
	"bytes"
	"strings"
	"testing"
)

func capture(fn func()) string {
	var buf bytes.Buffer
	old := Out
	Out = &buf
	fn()
	Out = old
	return buf.String()
}

func TestSuccessWithColor(t *testing.T) {
	NoColor = false
	Quiet = false
	out := capture(func() { Success("installed") })
	if !strings.Contains(out, "✓") {
		t.Errorf("expected checkmark, got: %q", out)
	}
	if !strings.Contains(out, "installed") {
		t.Errorf("expected message, got: %q", out)
	}
	if !strings.Contains(out, green) {
		t.Errorf("expected green ANSI, got: %q", out)
	}
}

func TestSuccessNoColor(t *testing.T) {
	NoColor = true
	Quiet = false
	out := capture(func() { Success("installed") })
	if strings.Contains(out, "\033[") {
		t.Errorf("expected no ANSI codes, got: %q", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("expected checkmark, got: %q", out)
	}
	NoColor = false
}

func TestErrorWithHint(t *testing.T) {
	NoColor = true
	Quiet = false
	out := capture(func() { Error("not found", "run install first") })
	if !strings.Contains(out, "✗") {
		t.Errorf("expected error symbol, got: %q", out)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected message, got: %q", out)
	}
	if !strings.Contains(out, "run install first") {
		t.Errorf("expected hint, got: %q", out)
	}
	NoColor = false
}

func TestErrorWithoutHint(t *testing.T) {
	NoColor = true
	out := capture(func() { Error("fail", "") })
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line with no hint, got %d: %q", len(lines), out)
	}
	NoColor = false
}

func TestWarning(t *testing.T) {
	NoColor = true
	Quiet = false
	out := capture(func() { Warning("stale snapshot") })
	if !strings.Contains(out, "–") {
		t.Errorf("expected dash symbol, got: %q", out)
	}
	if !strings.Contains(out, "stale snapshot") {
		t.Errorf("expected message, got: %q", out)
	}
	NoColor = false
}

func TestDim(t *testing.T) {
	NoColor = true
	Quiet = false
	out := capture(func() { Dim("some path") })
	if !strings.Contains(out, "some path") {
		t.Errorf("expected message, got: %q", out)
	}
	NoColor = false
}

func TestLine(t *testing.T) {
	NoColor = true
	Quiet = false
	out := capture(func() { Line() })
	if !strings.Contains(out, "─") {
		t.Errorf("expected separator, got: %q", out)
	}
	NoColor = false
}

func TestSection(t *testing.T) {
	NoColor = true
	Quiet = false
	out := capture(func() { Section("Snapshot", ".bifrost/session.md") })
	if !strings.Contains(out, "Snapshot") {
		t.Errorf("expected label, got: %q", out)
	}
	if !strings.Contains(out, ".bifrost/session.md") {
		t.Errorf("expected value, got: %q", out)
	}
	// Check 2-space indent
	if !strings.HasPrefix(out, "  ") {
		t.Errorf("expected 2-space indent, got: %q", out)
	}
	NoColor = false
}

func TestHeader(t *testing.T) {
	NoColor = true
	Quiet = false
	out := capture(func() { Header("Bifrost Briefing") })
	if !strings.Contains(out, "Bifrost Briefing") {
		t.Errorf("expected title, got: %q", out)
	}
	// Should have separator lines
	if strings.Count(out, "─") < 10 {
		t.Errorf("expected separator lines, got: %q", out)
	}
	NoColor = false
}

func TestQuietSuppresses(t *testing.T) {
	NoColor = true
	Quiet = true
	out := capture(func() {
		Success("should not appear")
		Warning("should not appear")
		Dim("should not appear")
		Line()
		Blank()
		Section("X", "Y")
		Header("title")
		Plain("nope")
	})
	if out != "" {
		t.Errorf("expected empty output in quiet mode, got: %q", out)
	}
	Quiet = false
	NoColor = false
}

func TestQuietAllowsErrors(t *testing.T) {
	NoColor = true
	Quiet = true
	out := capture(func() { Error("bad", "fix it") })
	if !strings.Contains(out, "bad") {
		t.Errorf("errors should print in quiet mode, got: %q", out)
	}
	Quiet = false
	NoColor = false
}

func TestBlank(t *testing.T) {
	Quiet = false
	out := capture(func() { Blank() })
	if out != "\n" {
		t.Errorf("expected single newline, got: %q", out)
	}
}

func TestIndentation(t *testing.T) {
	NoColor = true
	Quiet = false
	// All output should start with 2-space indent
	funcs := []struct {
		name string
		fn   func()
	}{
		{"Success", func() { Success("msg") }},
		{"Error", func() { Error("msg", "") }},
		{"Warning", func() { Warning("msg") }},
		{"Line", func() { Line() }},
		{"Section", func() { Section("L", "V") }},
		{"Header", func() { Header("T") }},
		{"Plain", func() { Plain("msg") }},
	}
	for _, f := range funcs {
		out := capture(f.fn)
		if !strings.HasPrefix(out, "  ") {
			t.Errorf("%s: expected 2-space indent, got: %q", f.name, out)
		}
	}
	NoColor = false
}
