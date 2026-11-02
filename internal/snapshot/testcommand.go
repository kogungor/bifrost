package snapshot

import (
	"regexp"
	"strings"
)

// shellSegmentRE splits a command line into the commands it runs.
var shellSegmentRE = regexp.MustCompile(`&&|\|\||;|\||\n`)

// commandPrefixRE matches what may precede the program in a segment: env
// assignments and wrappers such as `npx` or `uv run`.
var commandPrefixRE = regexp.MustCompile(`^(?:(?:[A-Za-z_][A-Za-z0-9_]*=\S*|sudo|time|env|npx|bunx|pnpm\s+exec|yarn\s+exec|uv\s+run|poetry\s+run|pipenv\s+run|bundle\s+exec)\s+)+`)

// testRunnerRE matches a segment that starts a test run.
var testRunnerRE = regexp.MustCompile(`^(?:` +
	`(?:go|cargo|dotnet|swift|mix|deno|bun|bazel|bazelisk|rake|mvn|\./mvnw|gradle|\./gradlew)\s+(?:\S+\s+)*?test\b` +
	`|(?:npm|pnpm|yarn)\s+(?:run\s+)?test\b|npm\s+t\b` +
	`|make\s+\S*test\S*` +
	`|python3?\s+-m\s+(?:pytest|unittest)\b` +
	`|(?:pytest|py\.test|jest|vitest|mocha|ava|rspec|phpunit|ctest|tox|nox|playwright\s+test)\b` +
	`)`)

// LooksLikeTestCommand reports whether a shell command runs a test suite, as
// opposed to merely mentioning one (`cat auth_test.go`, `ls tests/`).
func LooksLikeTestCommand(command string) bool {
	for _, segment := range shellSegmentRE.Split(command, -1) {
		segment = strings.TrimSpace(segment)
		segment = commandPrefixRE.ReplaceAllString(segment, "")
		if testRunnerRE.MatchString(segment) {
			return true
		}
	}
	return false
}
