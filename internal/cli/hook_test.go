package cli

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8(t *testing.T) {
	s := "ağaç" // multi-byte characters
	for n := 0; n <= len(s); n++ {
		got := truncateUTF8(s, n)
		if len(got) > n || !utf8.ValidString(got) {
			t.Errorf("truncateUTF8(%q, %d) = %q", s, n, got)
		}
	}
}
