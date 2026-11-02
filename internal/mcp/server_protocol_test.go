package mcp

import (
	"strings"
	"testing"
)

// modernMeta is the per-request _meta a 2026-07-28 client sends.
func modernMeta(version string) map[string]any {
	return map[string]any{
		"io.modelcontextprotocol/protocolVersion":    version,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "test", "version": "1"},
	}
}

func errorCode(t *testing.T, resp map[string]any) int {
	t.Helper()
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected JSON-RPC error, got %v", resp)
	}
	return int(errObj["code"].(float64))
}

func TestInitializeNegotiatesLegacyVersion(t *testing.T) {
	for requested, want := range map[string]string{
		"2025-06-18": "2025-06-18",
		"2024-11-05": "2024-11-05",
		"1999-01-01": "2025-11-25",
		"":           "2025-11-25",
	} {
		resps := runServer(t, t.TempDir(), rpc(1, "initialize", map[string]any{"protocolVersion": requested}))
		result := resps[0]["result"].(map[string]any)
		if result["protocolVersion"] != want {
			t.Errorf("initialize %q: protocolVersion = %v, want %s", requested, result["protocolVersion"], want)
		}
		if result["instructions"] == "" || result["instructions"] == nil {
			t.Errorf("initialize %q: missing instructions", requested)
		}
	}
}

func TestModernDiscover(t *testing.T) {
	resps := runServer(t, t.TempDir(), rpc(1, "server/discover", map[string]any{"_meta": modernMeta("2026-07-28")}))
	result, ok := resps[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("discover failed: %v", resps[0])
	}
	if result["resultType"] != "complete" {
		t.Errorf("resultType = %v, want complete", result["resultType"])
	}
	versions := result["supportedVersions"].([]any)
	if versions[0] != "2026-07-28" {
		t.Errorf("supportedVersions = %v, want 2026-07-28 first", versions)
	}
	meta := result["_meta"].(map[string]any)
	if info := meta["io.modelcontextprotocol/serverInfo"].(map[string]any); info["name"] != "bifrost" {
		t.Errorf("serverInfo = %v", info)
	}
	if _, ok := result["capabilities"].(map[string]any)["tools"]; !ok {
		t.Errorf("capabilities missing tools: %v", result["capabilities"])
	}
}

func TestModernUnsupportedVersion(t *testing.T) {
	resps := runServer(t, t.TempDir(), rpc(1, "tools/list", map[string]any{"_meta": modernMeta("1900-01-01")}))
	if code := errorCode(t, resps[0]); code != ErrCodeUnsupportedProtocolVersion {
		t.Fatalf("code = %d, want %d", code, ErrCodeUnsupportedProtocolVersion)
	}
	data := resps[0]["error"].(map[string]any)["data"].(map[string]any)
	if data["requested"] != "1900-01-01" || len(data["supported"].([]any)) == 0 {
		t.Errorf("unexpected error data: %v", data)
	}
}

func TestModernRequiresClientCapabilities(t *testing.T) {
	meta := map[string]any{"io.modelcontextprotocol/protocolVersion": "2026-07-28"}
	resps := runServer(t, t.TempDir(), rpc(1, "tools/list", map[string]any{"_meta": meta}))
	if code := errorCode(t, resps[0]); code != ErrCodeInvalidParams {
		t.Fatalf("code = %d, want %d", code, ErrCodeInvalidParams)
	}
}

func TestModernToolsListAndCall(t *testing.T) {
	resps := runServer(t, t.TempDir(),
		rpc(1, "tools/list", map[string]any{"_meta": modernMeta("2026-07-28")}),
		rpc(2, "tools/call", map[string]any{"_meta": modernMeta("2026-07-28"), "name": "bifrost_status"}),
	)
	for _, resp := range resps {
		result, ok := resp["result"].(map[string]any)
		if !ok || result["resultType"] != "complete" {
			t.Fatalf("modern result missing resultType: %v", resp)
		}
	}
	if resps[1]["result"].(map[string]any)["isError"] != false {
		t.Errorf("status call should succeed: %v", resps[1])
	}
}

func TestLegacyDiscoverIsNotModern(t *testing.T) {
	// A dual-era client probes with server/discover and falls back to
	// initialize on any non-modern error.
	resps := runServer(t, t.TempDir(), rpc(1, "server/discover", nil))
	if code := errorCode(t, resps[0]); code == ErrCodeUnsupportedProtocolVersion {
		t.Fatalf("discover without _meta must not look like a modern error")
	}
}

func TestPing(t *testing.T) {
	resps := runServer(t, t.TempDir(),
		rpc(1, "ping", nil),
		rpc(2, "ping", map[string]any{"_meta": modernMeta("2026-07-28")}),
	)
	for _, resp := range resps {
		if _, ok := resp["result"].(map[string]any); !ok {
			t.Fatalf("ping failed: %v", resp)
		}
	}
}

func TestToolValidationErrorIsToolResult(t *testing.T) {
	resps := runServer(t, t.TempDir(), rpc(1, "tools/call", map[string]any{
		"name":      "bifrost_write_snapshot",
		"arguments": map[string]any{"source_tool": "claude-code", "current_task": "x", "session_intent": "hacking"},
	}))
	if resps[0]["error"] != nil {
		t.Fatalf("validation errors must not be JSON-RPC errors: %v", resps[0])
	}
	result := resps[0]["result"].(map[string]any)
	if result["isError"] != true || !strings.Contains(toolErrorMessage(resps[0]), "session_intent") {
		t.Fatalf("expected isError result naming the bad field: %v", result)
	}
}

func TestUnknownToolIsProtocolError(t *testing.T) {
	resps := runServer(t, t.TempDir(), rpc(1, "tools/call", map[string]any{"name": "bifrost_nope"}))
	if code := errorCode(t, resps[0]); code != ErrCodeInvalidParams {
		t.Fatalf("code = %d, want %d", code, ErrCodeInvalidParams)
	}
}

func TestToolAnnotations(t *testing.T) {
	defs := map[string]ToolDefinition{}
	for _, def := range NewToolSet(t.TempDir()).Definitions() {
		if def.Annotations == nil || def.Title == "" {
			t.Errorf("%s has no annotations or title", def.Name)
			continue
		}
		if def.Annotations.OpenWorldHint {
			t.Errorf("%s only touches local files; openWorldHint must be false", def.Name)
		}
		defs[def.Name] = def
	}
	if !defs["bifrost_delete_plan"].Annotations.DestructiveHint {
		t.Error("bifrost_delete_plan must be destructive")
	}
	if !defs["bifrost_read_snapshot"].Annotations.ReadOnlyHint {
		t.Error("bifrost_read_snapshot must be read-only")
	}
	if defs["bifrost_write_snapshot"].Annotations.DestructiveHint {
		t.Error("bifrost_write_snapshot archives the previous snapshot; it is not destructive")
	}

	// The hints default to true when absent, so false values must be sent.
	resps := runServer(t, t.TempDir(), rpc(1, "tools/list", nil))
	tools := resps[0]["result"].(map[string]any)["tools"].([]any)
	annotations := tools[0].(map[string]any)["annotations"].(map[string]any)
	if v, ok := annotations["openWorldHint"]; !ok || v != false {
		t.Errorf("openWorldHint must be sent explicitly as false: %v", annotations)
	}
}

func TestOversizedMessageIsSkipped(t *testing.T) {
	saved := maxMessageBytes
	maxMessageBytes = 64
	defer func() { maxMessageBytes = saved }()

	long := rpc(1, "tools/call", map[string]any{"name": "bifrost_write_note", "arguments": map[string]any{"text": strings.Repeat("x", 500)}})
	resps := runServer(t, t.TempDir(), long, rpc(2, "ping", nil))
	if len(resps) != 2 {
		t.Fatalf("expected an error and the ping response, got %v", resps)
	}
	if code := errorCode(t, resps[0]); code != ErrCodeInvalidRequest {
		t.Errorf("oversized message code = %d, want %d", code, ErrCodeInvalidRequest)
	}
	if _, ok := resps[1]["result"].(map[string]any); !ok {
		t.Errorf("server should keep serving after an oversized message: %v", resps[1])
	}
}

func TestParseErrorHasNullID(t *testing.T) {
	resps := runServer(t, t.TempDir(), `{not json`)
	if code := errorCode(t, resps[0]); code != ErrCodeParseError {
		t.Errorf("code = %d, want %d", code, ErrCodeParseError)
	}
	if id, ok := resps[0]["id"]; !ok || id != nil {
		t.Errorf("parse error must carry \"id\": null, got %v", resps[0])
	}
}
