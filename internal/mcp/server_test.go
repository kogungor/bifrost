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

	if len(tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		tm := tool.(map[string]any)
		names[tm["name"].(string)] = true
	}
	for _, expected := range []string{"bifrost_read_snapshot", "bifrost_write_snapshot", "bifrost_write_note", "bifrost_status", "bifrost_read_plan", "bifrost_write_plan", "bifrost_update_plan", "bifrost_delete_plan", "bifrost_list_plans"} {
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

func TestWriteSnapshotPathTraversal(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_snapshot",
			"arguments": map[string]any{
				"source_tool":  "test",
				"current_task": "test",
				"active_files": []map[string]string{
					{"path": "../../../etc/passwd", "note": "traversal"},
				},
			},
		}),
	)

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for path traversal")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "invalid file path") {
		t.Errorf("expected 'invalid file path' error, got: %s", msg)
	}
}

func TestWriteSnapshotAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_snapshot",
			"arguments": map[string]any{
				"source_tool":  "test",
				"current_task": "test",
				"active_files": []map[string]string{
					{"path": "/etc/passwd", "note": "absolute"},
				},
			},
		}),
	)

	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for absolute path")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "invalid file path") {
		t.Errorf("expected 'invalid file path' error, got: %s", msg)
	}
}

func TestWriteSnapshotFieldTooLong(t *testing.T) {
	dir := t.TempDir()
	bigString := strings.Repeat("x", maxFieldLen+1)
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_snapshot",
			"arguments": map[string]any{
				"source_tool":  "test",
				"current_task": bigString,
			},
		}),
	)

	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for oversized field")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "exceeds") {
		t.Errorf("expected 'exceeds' error, got: %s", msg)
	}
}

func TestWriteNoteTooLong(t *testing.T) {
	dir := t.TempDir()
	bigText := strings.Repeat("x", maxNoteLen+1)
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_note",
			"arguments": map[string]any{
				"text": bigText,
				"from": "test",
			},
		}),
	)

	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for oversized note")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "exceeds") {
		t.Errorf("expected 'exceeds' error, got: %s", msg)
	}
}

func TestPlanWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Write a plan with a custom name
	writeResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test-tool",
				"title":       "Auth Refactor",
				"name":        "auth",
				"goal":        "Refactor authentication to JWT",
				"steps": []map[string]any{
					{"description": "Update middleware", "files": []string{"src/auth.go"}},
					{"description": "Add tests", "files": []string{"src/auth_test.go"}},
				},
				"constraints": []string{"No breaking changes"},
			},
		}),
	)

	writeResult := writeResps[0]["result"].(map[string]any)
	writeContent := writeResult["content"].([]any)
	writeText := writeContent[0].(map[string]any)
	var writeStatus map[string]any
	json.Unmarshal([]byte(writeText["text"].(string)), &writeStatus)
	if writeStatus["ok"] != true {
		t.Fatalf("write plan failed: %v", writeStatus)
	}
	if writeStatus["name"] != "auth" {
		t.Errorf("name = %v, want auth", writeStatus["name"])
	}

	// Read it back
	readResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_read_plan",
			"arguments": map[string]any{
				"name": "auth",
			},
		}),
	)

	readResult := readResps[0]["result"].(map[string]any)
	readContent := readResult["content"].([]any)
	readText := readContent[0].(map[string]any)
	var readData map[string]any
	json.Unmarshal([]byte(readText["text"].(string)), &readData)

	if readData["found"] != true {
		t.Fatalf("plan not found after write")
	}

	plan := readData["plan"].(map[string]any)
	if plan["title"] != "Auth Refactor" {
		t.Errorf("title = %v, want 'Auth Refactor'", plan["title"])
	}
	if plan["goal"] != "Refactor authentication to JWT" {
		t.Errorf("goal = %v, want 'Refactor authentication to JWT'", plan["goal"])
	}
	if plan["status"] != "draft" {
		t.Errorf("status = %v, want draft", plan["status"])
	}
	if plan["completion_pct"] != float64(0) {
		t.Errorf("completion_pct = %v, want 0", plan["completion_pct"])
	}

	steps := plan["steps"].([]any)
	if len(steps) != 2 {
		t.Errorf("steps count = %d, want 2", len(steps))
	}

	// Verify created_at and updated_at are present
	if plan["created_at"] == nil {
		t.Error("created_at is nil")
	}
	if plan["updated_at"] == nil {
		t.Error("updated_at is nil")
	}
}

func TestPlanUpdateReviewNotes(t *testing.T) {
	dir := t.TempDir()

	// Write a plan first
	runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "claude-code",
				"title":       "Test Plan",
				"steps": []map[string]any{
					{"description": "Step one"},
					{"description": "Step two"},
				},
			},
		}),
	)

	// Update with review notes, step status, and plan status
	updateResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_update_plan",
			"arguments": map[string]any{
				"plan_status": "active",
				"review_notes": []map[string]string{
					{"from": "opencode", "text": "Consider edge cases"},
				},
				"step_updates": []map[string]any{
					{"index": 0, "status": "done"},
				},
			},
		}),
	)

	updateResult := updateResps[0]["result"].(map[string]any)
	updateContent := updateResult["content"].([]any)
	updateText := updateContent[0].(map[string]any)
	var updateStatus map[string]any
	json.Unmarshal([]byte(updateText["text"].(string)), &updateStatus)
	if updateStatus["ok"] != true {
		t.Fatalf("update plan failed: %v", updateStatus)
	}
	if updateStatus["status"] != "active" {
		t.Errorf("status = %v, want active", updateStatus["status"])
	}
	if updateStatus["completion_pct"] != float64(50) {
		t.Errorf("completion_pct = %v, want 50", updateStatus["completion_pct"])
	}

	// Read back and verify
	readResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_read_plan",
		}),
	)

	readResult := readResps[0]["result"].(map[string]any)
	readContent := readResult["content"].([]any)
	readText := readContent[0].(map[string]any)
	var readData map[string]any
	json.Unmarshal([]byte(readText["text"].(string)), &readData)

	plan := readData["plan"].(map[string]any)

	// Check plan status
	if plan["status"] != "active" {
		t.Errorf("plan status = %v, want active", plan["status"])
	}

	// Check step status updated
	steps := plan["steps"].([]any)
	step0 := steps[0].(map[string]any)
	if step0["status"] != "done" {
		t.Errorf("step[0].status = %v, want done", step0["status"])
	}

	// Check review note added
	notes := plan["review_notes"].([]any)
	if len(notes) != 1 {
		t.Fatalf("review_notes count = %d, want 1", len(notes))
	}
	note0 := notes[0].(map[string]any)
	if note0["from"] != "opencode" {
		t.Errorf("note[0].from = %v, want opencode", note0["from"])
	}

	// Check completion tracking
	if plan["completion_pct"] != float64(50) {
		t.Errorf("completion_pct = %v, want 50", plan["completion_pct"])
	}
	if plan["steps_done"] != float64(1) {
		t.Errorf("steps_done = %v, want 1", plan["steps_done"])
	}
}

func TestPlanUpdateStepContent(t *testing.T) {
	dir := t.TempDir()

	// Write a plan
	runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test",
				"title":       "Step Edit Test",
				"steps": []map[string]any{
					{"description": "Original description", "files": []string{"old.go"}},
				},
			},
		}),
	)

	// Update step description and files
	runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_update_plan",
			"arguments": map[string]any{
				"step_updates": []map[string]any{
					{"index": 0, "description": "Updated description", "files": []string{"new.go", "other.go"}},
				},
			},
		}),
	)

	// Read back
	readResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_read_plan",
		}),
	)
	readResult := readResps[0]["result"].(map[string]any)
	readContent := readResult["content"].([]any)
	readText := readContent[0].(map[string]any)
	var readData map[string]any
	json.Unmarshal([]byte(readText["text"].(string)), &readData)

	plan := readData["plan"].(map[string]any)
	steps := plan["steps"].([]any)
	step0 := steps[0].(map[string]any)

	if step0["description"] != "Updated description" {
		t.Errorf("description = %v, want 'Updated description'", step0["description"])
	}
	files := step0["files"].([]any)
	if len(files) != 2 {
		t.Errorf("files count = %d, want 2", len(files))
	}
}

func TestPlanDelete(t *testing.T) {
	dir := t.TempDir()

	// Write a plan
	runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test",
				"title":       "To Delete",
				"name":        "deleteme",
			},
		}),
	)

	// Delete it
	deleteResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_delete_plan",
			"arguments": map[string]any{
				"name": "deleteme",
			},
		}),
	)
	deleteResult := deleteResps[0]["result"].(map[string]any)
	deleteContent := deleteResult["content"].([]any)
	deleteText := deleteContent[0].(map[string]any)
	var deleteData map[string]any
	json.Unmarshal([]byte(deleteText["text"].(string)), &deleteData)
	if deleteData["ok"] != true {
		t.Fatalf("delete failed: %v", deleteData)
	}

	// Verify it's gone
	readResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_read_plan",
			"arguments": map[string]any{
				"name": "deleteme",
			},
		}),
	)
	readResult := readResps[0]["result"].(map[string]any)
	readContent := readResult["content"].([]any)
	readText := readContent[0].(map[string]any)
	var readData map[string]any
	json.Unmarshal([]byte(readText["text"].(string)), &readData)
	if readData["found"] != false {
		t.Error("expected found=false after delete")
	}
}

func TestPlanDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_delete_plan",
			"arguments": map[string]any{
				"name": "nonexistent",
			},
		}),
	)
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for deleting nonexistent plan")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Errorf("expected 'not found' error, got: %s", msg)
	}
}

func TestPlanNameTraversal(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test",
				"title":       "evil",
				"name":        "../../../etc/evil",
			},
		}),
	)
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for path traversal plan name")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "invalid plan name") {
		t.Errorf("expected 'invalid plan name' error, got: %s", msg)
	}
}

func TestPlanUpdateInvalidStepIndex(t *testing.T) {
	dir := t.TempDir()

	// Write a plan with 1 step
	runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test",
				"title":       "Test",
				"steps":       []map[string]any{{"description": "Only step"}},
			},
		}),
	)

	// Try to update step at invalid index
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_update_plan",
			"arguments": map[string]any{
				"step_updates": []map[string]any{
					{"index": 5, "status": "done"},
				},
			},
		}),
	)
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for out-of-range step index")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "out of range") {
		t.Errorf("expected 'out of range' error, got: %s", msg)
	}
}

func TestPlanUpdateInvalidStepStatus(t *testing.T) {
	dir := t.TempDir()

	runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test",
				"title":       "Test",
				"steps":       []map[string]any{{"description": "Step"}},
			},
		}),
	)

	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_update_plan",
			"arguments": map[string]any{
				"step_updates": []map[string]any{
					{"index": 0, "status": "invalid_status"},
				},
			},
		}),
	)
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for invalid step status")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "invalid step status") {
		t.Errorf("expected 'invalid step status' error, got: %s", msg)
	}
}

func TestPlanUpdateInvalidPlanStatus(t *testing.T) {
	dir := t.TempDir()

	runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test",
				"title":       "Test",
			},
		}),
	)

	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_update_plan",
			"arguments": map[string]any{
				"plan_status": "invalid_lifecycle",
			},
		}),
	)
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for invalid plan status")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "invalid plan status") {
		t.Errorf("expected 'invalid plan status' error, got: %s", msg)
	}
}

func TestListPlansIntegration(t *testing.T) {
	dir := t.TempDir()

	// List with no plans
	listResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_list_plans",
		}),
	)

	listResult := listResps[0]["result"].(map[string]any)
	listContent := listResult["content"].([]any)
	listText := listContent[0].(map[string]any)
	var listData map[string]any
	json.Unmarshal([]byte(listText["text"].(string)), &listData)

	plans := listData["plans"].([]any)
	if len(plans) != 0 {
		t.Errorf("expected 0 plans, got %d", len(plans))
	}

	// Create two plans
	for _, name := range []string{"plan", "auth"} {
		runServer(t, dir,
			rpc(1, "tools/call", map[string]any{
				"name": "bifrost_write_plan",
				"arguments": map[string]any{
					"source_tool": "test",
					"title":       name + " title",
					"name":        name,
				},
			}),
		)
	}

	// List again
	listResps2 := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_list_plans",
		}),
	)

	listResult2 := listResps2[0]["result"].(map[string]any)
	listContent2 := listResult2["content"].([]any)
	listText2 := listContent2[0].(map[string]any)
	var listData2 map[string]any
	json.Unmarshal([]byte(listText2["text"].(string)), &listData2)

	plans2 := listData2["plans"].([]any)
	if len(plans2) != 2 {
		t.Errorf("expected 2 plans, got %d", len(plans2))
	}

	// Verify plan info includes status and completion
	for _, p := range plans2 {
		pMap := p.(map[string]any)
		if pMap["status"] == nil {
			t.Error("plan info missing status")
		}
		if pMap["completion_pct"] == nil {
			t.Error("plan info missing completion_pct")
		}
		if pMap["title"] == nil {
			t.Error("plan info missing title")
		}
	}
}

func TestPlanWriteFilePathTraversal(t *testing.T) {
	dir := t.TempDir()
	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test",
				"title":       "Evil Plan",
				"steps": []map[string]any{
					{"description": "Evil step", "files": []string{"../../../etc/passwd"}},
				},
			},
		}),
	)
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for path traversal in step files")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "invalid file path") {
		t.Errorf("expected 'invalid file path' error, got: %s", msg)
	}
}

func TestPlanUpdateEmptyReviewNote(t *testing.T) {
	dir := t.TempDir()

	runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test",
				"title":       "Test",
			},
		}),
	)

	resps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_update_plan",
			"arguments": map[string]any{
				"review_notes": []map[string]string{
					{"from": "", "text": "Missing from"},
				},
			},
		}),
	)
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error for empty review note fields")
	}
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "required") {
		t.Errorf("expected 'required' error, got: %s", msg)
	}
}

func TestPlanTimestampPreservation(t *testing.T) {
	dir := t.TempDir()

	// Write a plan
	writeResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_write_plan",
			"arguments": map[string]any{
				"source_tool": "test",
				"title":       "Timestamp Test",
			},
		}),
	)
	writeResult := writeResps[0]["result"].(map[string]any)
	writeContent := writeResult["content"].([]any)
	writeText := writeContent[0].(map[string]any)
	var writeData map[string]any
	json.Unmarshal([]byte(writeText["text"].(string)), &writeData)
	originalCreatedAt := writeData["created_at"].(string)

	// Update the plan
	runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_update_plan",
			"arguments": map[string]any{
				"plan_status": "active",
			},
		}),
	)

	// Read back and verify created_at is preserved
	readResps := runServer(t, dir,
		rpc(1, "tools/call", map[string]any{
			"name": "bifrost_read_plan",
		}),
	)
	readResult := readResps[0]["result"].(map[string]any)
	readContent := readResult["content"].([]any)
	readText := readContent[0].(map[string]any)
	var readData map[string]any
	json.Unmarshal([]byte(readText["text"].(string)), &readData)

	plan := readData["plan"].(map[string]any)
	if plan["created_at"] != originalCreatedAt {
		t.Errorf("created_at changed after update: %v -> %v", originalCreatedAt, plan["created_at"])
	}
}
