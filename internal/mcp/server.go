package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// JSON-RPC error codes
const (
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeAppError       = -32000
)

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any          `json:"result,omitempty"`
	Error   *RPCError    `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// InitializeResult is returned from the initialize method.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

// ServerInfo identifies this MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Capabilities advertises what the server supports.
type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability indicates tool support.
type ToolsCapability struct{}

// Serve runs the MCP server, reading JSON-RPC from in and writing to out.
func Serve(projectRoot, version string, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Allow up to 1MB per line
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(out)

	tools := NewToolSet(projectRoot)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp := Response{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: ErrCodeInvalidRequest, Message: "parse error"},
			}
			encoder.Encode(resp)
			continue
		}

		if req.JSONRPC != "2.0" {
			resp := Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &RPCError{Code: ErrCodeInvalidRequest, Message: "invalid jsonrpc version"},
			}
			encoder.Encode(resp)
			continue
		}

		resp := dispatch(req, tools, version)
		// Notifications (no ID) get no response
		if req.ID == nil {
			continue
		}
		encoder.Encode(resp)
	}

	return scanner.Err()
}

func dispatch(req Request, tools *ToolSet, version string) Response {
	switch req.Method {
	case "initialize":
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: InitializeResult{
				ProtocolVersion: "2024-11-05",
				ServerInfo: ServerInfo{
					Name:    "bifrost",
					Version: version,
				},
				Capabilities: Capabilities{
					Tools: &ToolsCapability{},
				},
			},
		}

	case "notifications/initialized":
		// No-op acknowledgement; no response for notifications
		return Response{JSONRPC: "2.0"}

	case "tools/list":
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": tools.Definitions(),
			},
		}

	case "tools/call":
		return handleToolCall(req, tools)

	default:
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: ErrCodeMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

// toolCallParams are the params for tools/call.
type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func handleToolCall(req Request, tools *ToolSet) Response {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: ErrCodeInvalidParams, Message: "invalid params: " + err.Error()},
		}
	}

	result, err := tools.Call(params.Name, params.Arguments)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: ErrCodeAppError, Message: err.Error()},
		}
	}

	// MCP tools/call returns content array
	content, _ := json.Marshal(result)
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": string(content)},
			},
		},
	}
}
