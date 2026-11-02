package snapshot

import (
	"os"
	"strings"
	"testing"
)

func TestWriteNoteRedactsSecrets(t *testing.T) {
	root := t.TempDir()
	secret := "sk_" + "live_" + strings.Repeat("a1B2c3D4", 4)
	if err := WriteNote(root, &HandoffNote{Timestamp: "2026-11-02T10:00:00Z", From: "claude-code", Text: "use key " + secret}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(NotePath(root))
	if strings.Contains(string(data), secret) {
		t.Fatalf("handoff note written with a secret:\n%s", data)
	}
}
