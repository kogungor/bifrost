package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// rpc builds a JSON-RPC request line.
func rpc(id int, method string, params any) string {
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		b, _ := json.Marshal(params)
		req["params"] = json.RawMessage(b)
	}
	line, _ := json.Marshal(req)
	return string(line)
}

// notification builds a JSON-RPC notification (no id).
func notification(method string) string {
	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	line, _ := json.Marshal(req)
	return string(line)
}

func runServer(t *testing.T, projectRoot string, lines ...string) []map[string]any {
	t.Helper()
	input := strings.Join(lines, "\n") + "\n"
	var out bytes.Buffer
	err := Serve(projectRoot, "test", strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Serve error: %v", err)
	}

	var responses []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("unmarshal response: %v\nline: %s", err, line)
		}
		responses = append(responses, resp)
	}
	return responses
}

func TestInitialize(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir, rpc(1, "initialize", map[string]any{}))

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}

	resp := resps[0]
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in response: %v", resp)
	}

	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("no serverInfo: %v", result)
	}

	if info["name"] != "bifrost" {
		t.Errorf("server name = %v, want bifrost", info["name"])
	}
}

func TestToolsList(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir, rpc(1, "tools/list", nil))

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}

	result, ok := resps[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", resps[0])
	}

	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("no tools array: %v", result)
	}

	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		tm := tool.(map[string]any)
		names[tm["name"].(string)] = true
	}
	for _, expected := range []string{"bifrost_read_snapshot", "bifrost_write_snapshot", "bifrost_write_note", "bifrost_status"} {
		if !names[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}
}

func TestStatusEmpty(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_status",
		}),
	)

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}

	result := resps[0]["result"].(map[string]any)
	content := result["content"].([]any)
	textObj := content[0].(map[string]any)

	var status map[string]any
	if err := json.Unmarshal([]byte(textObj["text"].(string)), &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}

	if status["has_snapshot"] != false {
		t.Errorf("has_snapshot = %v, want false", status["has_snapshot"])
	}
	if status["history_count"] != float64(0) {
		t.Errorf("history_count = %v, want 0", status["history_count"])
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Write a snapshot
	writeResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_snapshot",
			"arguments": map[string]any{
				"source_tool":    "test-tool",
				"token_pressure": "medium",
				"current_task":   "implement MCP server",
				"status":         []string{"- [x] server.go", "- [ ] tests"},
				"active_files": []map[string]string{
					{"path": "internal/mcp/server.go", "note": "main server"},
				},
				"decisions":         []string{"- use stdio JSON-RPC"},
				"environment_notes": []string{"- Go 1.23"},
				"next_step":         "write tests",
			},
		}),
	)

	if len(writeResps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(writeResps))
	}

	writeResult := writeResps[0]["result"].(map[string]any)
	writeContent := writeResult["content"].([]any)
	writeText := writeContent[0].(map[string]any)
	var writeStatus map[string]any
	json.Unmarshal([]byte(writeText["text"].(string)), &writeStatus)
	if writeStatus["ok"] != true {
		t.Fatalf("write failed: %v", writeStatus)
	}

	// Read it back
	readResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_read_snapshot",
		}),
	)

	if len(readResps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(readResps))
	}

	readResult := readResps[0]["result"].(map[string]any)
	readContent := readResult["content"].([]any)
	readText := readContent[0].(map[string]any)
	var readData map[string]any
	json.Unmarshal([]byte(readText["text"].(string)), &readData)

	if readData["found"] != true {
		t.Fatalf("snapshot not found after write")
	}

	snap := readData["snapshot"].(map[string]any)
	if snap["source_tool"] != "test-tool" {
		t.Errorf("source_tool = %v, want test-tool", snap["source_tool"])
	}
	if snap["current_task"] != "implement MCP server" {
		t.Errorf("current_task = %v, want 'implement MCP server'", snap["current_task"])
	}
	if snap["token_pressure"] != "medium" {
		t.Errorf("token_pressure = %v, want medium", snap["token_pressure"])
	}
}

func TestUnknownMethod(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir, rpc(1, "unknown/method", nil))

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}

	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error response: %v", resps[0])
	}

	code := errObj["code"].(float64)
	if int(code) != ErrCodeMethodNotFound {
		t.Errorf("error code = %v, want %d", code, ErrCodeMethodNotFound)
	}
}

func TestInvalidParams(t *testing.T) {
	dir := t.TempDir()
	// tools/call with invalid JSON params
	line := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"not-an-object"}`
	resps := runServer(t, dir, line)

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}

	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error response: %v", resps[0])
	}

	code := errObj["code"].(float64)
	if int(code) != ErrCodeInvalidParams {
		t.Errorf("error code = %v, want %d", code, ErrCodeInvalidParams)
	}
}

func TestNotificationNoResponse(t *testing.T) {
	dir := t.TempDir()
	// Notifications (no id) should produce no response
	resps := runServer(t, dir, notification("notifications/initialized"))

	if len(resps) != 0 {
		t.Errorf("expected 0 responses for notification, got %d", len(resps))
	}
}

func TestReadSnapshotNotFound(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_read_snapshot",
		}),
	)

	readResult := resps[0]["result"].(map[string]any)
	readContent := readResult["content"].([]any)
	readText := readContent[0].(map[string]any)
	var readData map[string]any
	json.Unmarshal([]byte(readText["text"].(string)), &readData)

	if readData["found"] != false {
		t.Errorf("expected found=false for empty project")
	}
}
