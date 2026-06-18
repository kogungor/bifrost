package security

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Config struct {
	Strict            bool
	RedactBeforeWrite bool
	Allowlist         []string
}

type fileConfig struct {
	Security struct {
		Strict            bool     `json:"strict"`
		RedactBeforeWrite *bool    `json:"redact_before_write"`
		Allowlist         []string `json:"allowlist"`
	} `json:"security"`
}

type Finding struct {
	Kind        string
	Start       int
	End         int
	Allowlisted bool
	replacement string
	raw         string
}

func DefaultConfig() Config {
	return Config{RedactBeforeWrite: true}
}

func LoadConfig(projectRoot string) Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(filepath.Join(projectRoot, ".bifrost", "config.json"))
	if err != nil {
		return cfg
	}
	var parsed fileConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		return cfg
	}
	cfg.Strict = parsed.Security.Strict
	cfg.Allowlist = parsed.Security.Allowlist
	if parsed.Security.RedactBeforeWrite != nil {
		cfg.RedactBeforeWrite = *parsed.Security.RedactBeforeWrite
	}
	return cfg
}

func ScanString(text string, cfg Config) []Finding {
	var findings []Finding
	for _, detector := range detectors {
		matches := detector.re.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			raw := text[start:end]
			replacement := "[REDACTED:" + detector.kind + "]"
			if detector.redact != nil {
				replacement = detector.redact(text, match)
			}
			findings = append(findings, Finding{
				Kind:        detector.kind,
				Start:       start,
				End:         end,
				raw:         raw,
				replacement: replacement,
				Allowlisted: isAllowlisted(detector.kind, raw, cfg.Allowlist),
			})
		}
	}
	findings = append(findings, scanEntropyContext(text, cfg.Allowlist)...)
	return compactFindings(findings)
}

func RedactString(text string, cfg Config) (string, []Finding) {
	findings := ScanString(text, cfg)
	if len(findings) == 0 {
		return text, nil
	}
	var b strings.Builder
	cursor := 0
	for _, finding := range findings {
		if finding.Allowlisted {
			continue
		}
		if finding.Start < cursor {
			continue
		}
		b.WriteString(text[cursor:finding.Start])
		b.WriteString(finding.replacement)
		cursor = finding.End
	}
	if cursor == 0 {
		return text, findings
	}
	b.WriteString(text[cursor:])
	return b.String(), findings
}

func CountActive(findings []Finding) int {
	count := 0
	for _, finding := range findings {
		if !finding.Allowlisted {
			count++
		}
	}
	return count
}

func CountAllowlisted(findings []Finding) int {
	count := 0
	for _, finding := range findings {
		if finding.Allowlisted {
			count++
		}
	}
	return count
}

func Summary(findings []Finding) string {
	counts := map[string]int{}
	for _, finding := range findings {
		if finding.Allowlisted {
			continue
		}
		counts[finding.Kind]++
	}
	if len(counts) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}

type detector struct {
	kind   string
	re     *regexp.Regexp
	redact func(text string, match []int) string
}

var detectors = []detector{
	{
		kind: "private_key",
		re:   regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`),
	},
	{
		kind: "anthropic_key",
		re:   regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}\b`),
	},
	{
		kind: "openai_key",
		re:   regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_-]{20,}\b`),
	},
	{
		kind: "github_token",
		re:   regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{30,}\b`),
	},
	{
		kind: "aws_key",
		re:   regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
	},
	{
		kind: "bearer_token",
		re:   regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{20,}`),
		redact: func(text string, match []int) string {
			return "Bearer [REDACTED:bearer_token]"
		},
	},
	{
		kind: "jwt",
		re:   regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`),
	},
	{
		kind: "database_url",
		re:   regexp.MustCompile(`(?i)\b(?:postgres|postgresql|mysql|mongodb(?:\+srv)?)://[^"\s]+:[^"\s]+@[^"\s]+`),
	},
	{
		kind: "env_secret",
		re:   regexp.MustCompile(`(?i)\b([A-Z_][A-Z0-9_]*(?:SECRET|TOKEN|KEY|PASSWORD|PASS|DATABASE_URL|DB_URL|API_KEY)[A-Z0-9_]*\s*=\s*)([^\s"'` + "`" + `]+)`),
		redact: func(text string, match []int) string {
			prefix := text[match[2]:match[3]]
			return prefix + "[REDACTED:env_secret]"
		},
	},
}

var entropyContextRE = regexp.MustCompile(`(?i)\b(api[_-]?key|secret|token|password)\b\s*[:=]\s*["']?([A-Za-z0-9+/=_-]{32,})["']?`)

func scanEntropyContext(text string, allowlist []string) []Finding {
	matches := entropyContextRE.FindAllStringSubmatchIndex(text, -1)
	findings := make([]Finding, 0, len(matches))
	for _, match := range matches {
		value := text[match[4]:match[5]]
		if shannonEntropy(value) < 4.0 {
			continue
		}
		raw := text[match[0]:match[1]]
		replacement := text[match[0]:match[4]] + "[REDACTED:high_entropy_secret]"
		findings = append(findings, Finding{
			Kind:        "high_entropy_secret",
			Start:       match[0],
			End:         match[1],
			raw:         raw,
			replacement: replacement,
			Allowlisted: isAllowlisted("high_entropy_secret", raw, allowlist) || isAllowlisted("high_entropy_secret", value, allowlist),
		})
	}
	return findings
}

func compactFindings(findings []Finding) []Finding {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Start == findings[j].Start {
			return findings[i].End > findings[j].End
		}
		return findings[i].Start < findings[j].Start
	})
	compacted := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		if len(compacted) == 0 {
			compacted = append(compacted, finding)
			continue
		}
		last := &compacted[len(compacted)-1]
		if finding.Start < last.End {
			if finding.End-finding.Start > last.End-last.Start {
				*last = finding
			}
			continue
		}
		compacted = append(compacted, finding)
	}
	return compacted
}

func isAllowlisted(kind, raw string, allowlist []string) bool {
	for _, item := range allowlist {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if item == kind || strings.Contains(raw, item) {
			return true
		}
	}
	return false
}

func shannonEntropy(value string) float64 {
	if value == "" {
		return 0
	}
	counts := map[rune]float64{}
	for _, r := range value {
		counts[r]++
	}
	length := float64(len([]rune(value)))
	entropy := 0.0
	for _, count := range counts {
		p := count / length
		entropy -= p * math.Log2(p)
	}
	return entropy
}
