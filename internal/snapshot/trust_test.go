package snapshot

import (
	"strings"
	"testing"
	"time"
)

func TestTrustFromLegacyConfidenceKeepsImplementationSignal(t *testing.T) {
	trust := TrustFromLegacyConfidence("medium")
	if trust.Implementation != TrustMedium {
		t.Fatalf("implementation = %q, want medium", trust.Implementation)
	}
	if trust.Tests != TrustUnknown || trust.Security != TrustUnknown || trust.Architecture != TrustUnknown || trust.Freshness != TrustUnknown {
		t.Fatalf("legacy confidence should not invent non-implementation trust: %+v", trust)
	}
	if trust.Evidence != EvidenceWeak {
		t.Fatalf("legacy confidence evidence = %q, want weak", trust.Evidence)
	}
}

func TestTrustSummaryRoundTrip(t *testing.T) {
	trust := TrustV2{
		Implementation: TrustHigh,
		Tests:          TrustLow,
		Security:       TrustMedium,
		Architecture:   TrustUnknown,
		Freshness:      FreshnessStale,
		Evidence:       EvidenceStrong,
	}

	summary := TrustSummary(trust)
	parsed := TrustFromSummary(summary)
	if parsed != normalizeTrust(trust) {
		t.Fatalf("parsed trust = %+v, want %+v from %q", parsed, normalizeTrust(trust), summary)
	}
}

func TestApplyTrustModelV2DowngradesMissingEvidenceAndSecurityRisk(t *testing.T) {
	snap := &SnapshotV2{
		Interpretation: InterpretationV2{
			StatusItems: []StatusItemV2{{ID: "status_1", Text: "Tests pass", State: "claimed_done"}},
			Risks:       []RiskV2{{ID: "risk_1", Text: "Auth behavior unresolved", Severity: "high"}},
		},
		Observed: ObservedV2{
			Files: []FileObservedV2{{Path: "internal/auth/token.go", Exists: true}},
		},
		ActiveFiles: []ActiveFileV2{{
			Path: "internal/auth/token.go",
			Trust: TrustV2{
				Implementation: TrustHigh,
				Tests:          TrustHigh,
				Security:       TrustHigh,
				Architecture:   TrustHigh,
			},
		}},
	}

	ApplyTrustModelV2(snap)

	trust := snap.ActiveFiles[0].Trust
	if trust.Evidence != EvidenceWeak {
		t.Fatalf("evidence = %q, want weak", trust.Evidence)
	}
	if trust.Freshness != FreshnessCurrent {
		t.Fatalf("freshness = %q, want current", trust.Freshness)
	}
	if trust.Tests != TrustLow {
		t.Fatalf("tests = %q, want low without passing test evidence", trust.Tests)
	}
	if trust.Security != TrustLow {
		t.Fatalf("security = %q, want low for security-sensitive path without evidence", trust.Security)
	}
	if trust.Architecture != TrustLow {
		t.Fatalf("architecture = %q, want low for high unresolved risk without evidence", trust.Architecture)
	}
}

func TestApplyTrustModelV2UsesEvidenceAndPassingTests(t *testing.T) {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	snap := &SnapshotV2{
		Interpretation: InterpretationV2{
			StatusItems: []StatusItemV2{{ID: "status_1", Text: "Implementation done", State: "claimed_done"}},
		},
		Observed: ObservedV2{
			Files: []FileObservedV2{{Path: "src/service.go", Exists: true}},
		},
		Evidence: []EvidenceV2{
			{ID: "ev_file_1", Type: EvidenceTypeFileMetadata, Source: "collector.file", ObservedAt: now},
			{ID: "ev_test_1", Type: EvidenceTypeTestResult, Source: "reported.command", ObservedAt: now, Data: map[string]any{"command": "go test ./...", "exit_code": 0}},
		},
		ActiveFiles: []ActiveFileV2{{
			Path:         "src/service.go",
			Trust:        TrustV2{Implementation: TrustMedium},
			EvidenceRefs: []string{"ev_file_1"},
		}},
	}

	ApplyTrustModelV2(snap)

	trust := snap.ActiveFiles[0].Trust
	if trust.Evidence != EvidenceStrong {
		t.Fatalf("evidence = %q, want strong", trust.Evidence)
	}
	if trust.Tests != TrustMedium {
		t.Fatalf("tests = %q, want medium from passing test evidence", trust.Tests)
	}
	if trust.Freshness != FreshnessCurrent {
		t.Fatalf("freshness = %q, want current", trust.Freshness)
	}
}

func TestApplyTrustModelV2DoesNotUpgradeExplicitLowTestTrust(t *testing.T) {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	snap := &SnapshotV2{
		Observed: ObservedV2{
			Files: []FileObservedV2{{Path: "src/service.go", Exists: true}},
		},
		Evidence: []EvidenceV2{
			{ID: "ev_test_1", Type: EvidenceTypeTestResult, Source: "reported.command", ObservedAt: now, Data: map[string]any{"command": "go test ./...", "exit_code": 0}},
		},
		ActiveFiles: []ActiveFileV2{{
			Path:  "src/service.go",
			Trust: TrustV2{Implementation: TrustMedium, Tests: TrustLow},
		}},
	}

	ApplyTrustModelV2(snap)

	if got := snap.ActiveFiles[0].Trust.Tests; got != TrustLow {
		t.Fatalf("tests = %q, want explicit low trust to remain low", got)
	}
}

func TestRenderAndParseTrustSummary(t *testing.T) {
	s := &Snapshot{
		BifrostVersion: CurrentVersion,
		Timestamp:      time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		SourceTool:     "claude-code",
		Project:        "bifrost",
		TokenPressure:  "low",
		CurrentTask:    "trust render",
		ActiveFiles: []ActiveFile{{
			Path:       "src/auth.go",
			Note:       "security-sensitive implementation",
			Confidence: "implementation=medium, tests=low, security=low, architecture=medium, freshness=stale, evidence=weak",
		}},
	}

	rendered := Render(s)
	if !strings.Contains(rendered, "[trust: implementation=medium, tests=low, security=low, architecture=medium, freshness=stale, evidence=weak]") {
		t.Fatalf("rendered snapshot missing trust summary:\n%s", rendered)
	}
	parsed, err := Parse([]byte(rendered))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ActiveFiles[0].Confidence != s.ActiveFiles[0].Confidence {
		t.Fatalf("parsed trust = %q, want %q", parsed.ActiveFiles[0].Confidence, s.ActiveFiles[0].Confidence)
	}
}
