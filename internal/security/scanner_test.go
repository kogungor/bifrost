package security

import (
	"strings"
	"testing"
)

func TestScanStringDetectsKnownSecretFormats(t *testing.T) {
	text := strings.Join([]string{
		"Authorization: Bearer abcdefghijklmnopqrstuvwxyz123456",
		"OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz123456",
		"API_SECRET = abcdefghijklmnopqrstuvwxyz123456",
		"ANTHROPIC_API_KEY=sk-ant-abcdefghijklmnopqrstuvwxyz123456",
		"openai sk-proj-abcdefghijklmnopqrstuvwxyz123456",
		"anthropic sk-ant-abcdefghijklmnopqrstuvwxyz123456",
		"github=ghp_abcdefghijklmnopqrstuvwxyzABCDE1234567890",
		"db=postgres://user:password@localhost:5432/app",
		"jwt=eyJaaaaaaaaaaa.eyJbbbbbbbbbbb.cccccccccccccc",
		"aws=AKIA1234567890ABCDEF",
		"token: qwertyuiopASDFGHJKLzxcvbnm1234567890+/=",
		"-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
	}, "\n")

	findings := ScanString(text, DefaultConfig())
	for _, kind := range []string{
		"bearer_token",
		"openai_key",
		"anthropic_key",
		"env_secret",
		"github_token",
		"database_url",
		"jwt",
		"aws_key",
		"high_entropy_secret",
		"private_key",
	} {
		if !hasFindingKind(findings, kind) {
			t.Fatalf("missing finding kind %q in %#v", kind, findings)
		}
	}
}

func TestRedactStringPreservesPrefixesAndAllowlist(t *testing.T) {
	text := "Authorization: Bearer abcdefghijklmnopqrstuvwxyz123456\nPUBLIC_TOKEN=example-token\n"
	cfg := DefaultConfig()
	cfg.Allowlist = []string{"PUBLIC_TOKEN"}

	redacted, findings := RedactString(text, cfg)
	if CountActive(findings) != 1 || CountAllowlisted(findings) != 1 {
		t.Fatalf("counts active=%d allowlisted=%d findings=%#v", CountActive(findings), CountAllowlisted(findings), findings)
	}
	if !strings.Contains(redacted, "Bearer [REDACTED:bearer_token]") {
		t.Fatalf("bearer token was not redacted with prefix preserved: %s", redacted)
	}
	if !strings.Contains(redacted, "PUBLIC_TOKEN=example-token") {
		t.Fatalf("allowlisted value should remain unchanged: %s", redacted)
	}
}

func hasFindingKind(findings []Finding, kind string) bool {
	for _, finding := range findings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}
