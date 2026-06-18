package snapshot

import (
	"fmt"

	"github.com/kogungor/bifrost/internal/security"
)

func redactBeforeWrite(projectRoot, data string) (string, error) {
	cfg := security.LoadConfig(projectRoot)
	redacted, findings := security.RedactString(data, cfg)
	active := security.CountActive(findings)
	if active == 0 {
		return data, nil
	}
	if cfg.Strict {
		return "", fmt.Errorf("snapshot contains secret-like values: %s", security.Summary(findings))
	}
	if !cfg.RedactBeforeWrite {
		return data, nil
	}
	return redacted, nil
}
