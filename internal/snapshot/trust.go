package snapshot

import "strings"

const (
	TrustHigh    = "high"
	TrustMedium  = "medium"
	TrustLow     = "low"
	TrustUnknown = "unknown"

	FreshnessCurrent = "current"
	FreshnessStale   = "stale"

	EvidenceStrong = "strong"
	EvidenceMedium = "medium"
	EvidenceWeak   = "weak"
)

// ApplyTrustModelV2 fills missing trust dimensions and applies conservative
// downgrades based on evidence, observed file freshness, test evidence, and
// unresolved high-risk context.
func ApplyTrustModelV2(s *SnapshotV2) {
	if s == nil {
		return
	}
	evidenceByID := evidenceByID(s.Evidence)
	filesByPath := observedFilesByPath(s.Observed.Files)
	hasTests := hasTestEvidence(s.Evidence)
	hasHighRisk := hasHighSeverityRisk(s)

	for i := range s.ActiveFiles {
		file := &s.ActiveFiles[i]
		trust := normalizeTrust(file.Trust)
		trust.Evidence = evidenceTrust(file.EvidenceRefs, evidenceByID)
		trust.Freshness = fileFreshness(file.Path, filesByPath)
		if hasTests && trust.Tests == TrustUnknown {
			trust.Tests = TrustMedium
		} else if hasClaimedDoneStatus(s) {
			trust.Tests = TrustLow
		}
		if isSecuritySensitivePath(file.Path) {
			trust.Security = minTrust(maxUnknown(trust.Security, TrustMedium), TrustMedium)
			if trust.Evidence == EvidenceWeak {
				trust.Security = TrustLow
			}
		}
		if hasHighRisk {
			trust.Security = minTrust(maxUnknown(trust.Security, TrustMedium), TrustMedium)
			trust.Architecture = minTrust(maxUnknown(trust.Architecture, TrustMedium), TrustMedium)
			if trust.Evidence == EvidenceWeak {
				trust.Security = TrustLow
				trust.Architecture = TrustLow
			}
		}
		file.Trust = trust
	}
}

func TrustFromLegacyConfidence(confidence string) TrustV2 {
	level := normalizeTrustLevel(confidence)
	if level == "" {
		level = TrustUnknown
	}
	return TrustV2{
		Implementation: level,
		Tests:          TrustUnknown,
		Security:       TrustUnknown,
		Architecture:   TrustUnknown,
		Freshness:      TrustUnknown,
		Evidence:       EvidenceWeak,
	}
}

func TrustSummary(trust TrustV2) string {
	trust = normalizeTrust(trust)
	parts := []string{
		"implementation=" + trust.Implementation,
		"tests=" + trust.Tests,
		"security=" + trust.Security,
		"architecture=" + trust.Architecture,
		"freshness=" + trust.Freshness,
		"evidence=" + trust.Evidence,
	}
	return strings.Join(parts, ", ")
}

func TrustFromSummary(summary string) TrustV2 {
	var trust TrustV2
	for _, part := range strings.Split(summary, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "implementation":
			trust.Implementation = normalizeTrustLevel(value)
		case "tests":
			trust.Tests = normalizeTrustLevel(value)
		case "security":
			trust.Security = normalizeTrustLevel(value)
		case "architecture":
			trust.Architecture = normalizeTrustLevel(value)
		case "freshness":
			trust.Freshness = normalizeFreshness(value)
		case "evidence":
			trust.Evidence = normalizeEvidenceTrust(value)
		}
	}
	return normalizeTrust(trust)
}

func normalizeTrust(trust TrustV2) TrustV2 {
	trust.Implementation = defaultTrustLevel(trust.Implementation)
	trust.Tests = defaultTrustLevel(trust.Tests)
	trust.Security = defaultTrustLevel(trust.Security)
	trust.Architecture = defaultTrustLevel(trust.Architecture)
	trust.Freshness = normalizeFreshness(trust.Freshness)
	if trust.Freshness == "" {
		trust.Freshness = TrustUnknown
	}
	trust.Evidence = normalizeEvidenceTrust(trust.Evidence)
	if trust.Evidence == "" {
		trust.Evidence = EvidenceWeak
	}
	return trust
}

func defaultTrustLevel(value string) string {
	value = normalizeTrustLevel(value)
	if value == "" {
		return TrustUnknown
	}
	return value
}

func normalizeTrustLevel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if validTrustLevels[value] {
		return value
	}
	return ""
}

func normalizeFreshness(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if validFreshnessLevels[value] {
		return value
	}
	return ""
}

func normalizeEvidenceTrust(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == TrustHigh {
		return EvidenceStrong
	}
	if value == TrustLow {
		return EvidenceWeak
	}
	if validEvidenceTrustLevels[value] {
		return value
	}
	return ""
}

func evidenceTrust(refs []string, evidence map[string]EvidenceV2) string {
	if len(refs) == 0 {
		return EvidenceWeak
	}
	best := EvidenceWeak
	for _, ref := range refs {
		ev, ok := evidence[ref]
		if !ok {
			return EvidenceWeak
		}
		switch ev.Type {
		case EvidenceTypeGitStatus, EvidenceTypeFileMetadata, EvidenceTypeDiffSummary, EvidenceTypeCommandResult, EvidenceTypeTestResult, EvidenceTypeProjectMetadata:
			best = EvidenceStrong
		case EvidenceTypeManualNote:
			if best != EvidenceStrong {
				best = EvidenceMedium
			}
		}
	}
	return best
}

func fileFreshness(path string, files map[string]FileObservedV2) string {
	file, ok := files[path]
	if !ok {
		return TrustUnknown
	}
	if !file.Exists {
		return FreshnessStale
	}
	return FreshnessCurrent
}

func evidenceByID(evidence []EvidenceV2) map[string]EvidenceV2 {
	out := make(map[string]EvidenceV2, len(evidence))
	for _, ev := range evidence {
		out[ev.ID] = ev
	}
	return out
}

func observedFilesByPath(files []FileObservedV2) map[string]FileObservedV2 {
	out := make(map[string]FileObservedV2, len(files))
	for _, file := range files {
		out[file.Path] = file
	}
	return out
}

func hasClaimedDoneStatus(s *SnapshotV2) bool {
	for _, item := range s.Interpretation.StatusItems {
		if item.State == "claimed_done" || item.State == "verified_done" {
			return true
		}
	}
	return false
}

func hasHighSeverityRisk(s *SnapshotV2) bool {
	for _, risk := range s.Interpretation.Risks {
		if strings.EqualFold(risk.Severity, "high") {
			return true
		}
	}
	for _, question := range s.Interpretation.OpenQuestions {
		if strings.EqualFold(question.Severity, "high") {
			return true
		}
	}
	return false
}

func isSecuritySensitivePath(path string) bool {
	path = strings.ToLower(path)
	for _, marker := range []string{
		"auth", "token", "secret", "password", "passwd", "credential",
		"security", "crypto", "jwt", "oauth", "session", "cookie", ".env",
	} {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}

func minTrust(value, max string) string {
	if trustRank(value) > trustRank(max) {
		return max
	}
	return value
}

func maxUnknown(value, fallback string) string {
	if value == "" || value == TrustUnknown {
		return fallback
	}
	return value
}

func trustRank(value string) int {
	switch value {
	case TrustHigh:
		return 3
	case TrustMedium:
		return 2
	case TrustLow:
		return 1
	default:
		return 0
	}
}
