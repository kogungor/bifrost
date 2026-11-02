package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// JSON-RPC error codes
const (
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603
	// ErrCodeUnsupportedProtocolVersion is MCP's UnsupportedProtocolVersionError.
	ErrCodeUnsupportedProtocolVersion = -32022
)

// Protocol versions. In the modern revision every request carries its
// version in _meta and there is no initialize handshake; legacy revisions
// negotiate once through initialize. Bifrost serves both.
const modernProtocolVersion = "2026-07-28"

var legacyProtocolVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

// _meta keys defined by the modern revision.
const (
	metaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
	metaServerInfo         = "io.modelcontextprotocol/serverInfo"
)

// maxMessageBytes bounds one newline-delimited message. A larger message is
// answered with an error and skipped instead of stopping the server.
var maxMessageBytes = 16 << 20

const serverInstructions = "Bifrost stores AI coding handoffs for this project. " +
	"Call bifrost_read_snapshot when resuming work and bifrost_write_snapshot when handing off. " +
	"Use the bifrost_*_plan tools to create, review, and track implementation plans."

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response. ID is null when the request's ID
// could not be read.
type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Serve runs the MCP server, reading JSON-RPC from in and writing to out.
func Serve(projectRoot, version string, in io.Reader, out io.Writer) error {
	reader := bufio.NewReaderSize(in, 64*1024)
	encoder := json.NewEncoder(out)
	tools := NewToolSet(projectRoot)

	for {
		line, tooLarge, err := readMessage(reader, maxMessageBytes)
		if tooLarge {
			encoder.Encode(errorResponse(nil, ErrCodeInvalidRequest, fmt.Sprintf("message exceeds %d bytes", maxMessageBytes), nil))
		} else if len(bytes.TrimSpace(line)) > 0 {
			if resp, ok := handleMessage(line, tools, version); ok {
				encoder.Encode(resp)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// readMessage reads one newline-delimited message. A message longer than max
// is consumed and discarded, and tooLarge is reported instead.
func readMessage(r *bufio.Reader, max int) (line []byte, tooLarge bool, err error) {
	for {
		chunk, err := r.ReadSlice('\n')
		if !tooLarge {
			if len(line)+len(chunk) > max {
				tooLarge, line = true, nil
			} else {
				line = append(line, chunk...)
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return line, tooLarge, err
	}
}

// handleMessage returns the response for one message, and false for
// notifications, which get no response.
func handleMessage(line []byte, tools *ToolSet, version string) (Response, bool) {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return errorResponse(nil, ErrCodeParseError, "parse error", nil), true
	}
	if req.JSONRPC != "2.0" {
		return errorResponse(req.ID, ErrCodeInvalidRequest, "invalid jsonrpc version", nil), true
	}
	resp := dispatch(req, tools, version)
	if req.ID == nil {
		return Response{}, false
	}
	return resp, true
}

// dispatch routes a request by era: a request whose _meta names a protocol
// version is served statelessly under the modern revision; anything else
// follows the legacy initialize-based revisions.
func dispatch(req Request, tools *ToolSet, version string) Response {
	var params struct {
		Meta map[string]json.RawMessage `json:"_meta"`
	}
	_ = json.Unmarshal(req.Params, &params)
	if rawVersion, ok := params.Meta[metaProtocolVersion]; ok {
		return dispatchModern(req, params.Meta, rawVersion, tools, version)
	}

	switch req.Method {
	case "initialize":
		var init struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &init)
		return resultResponse(req.ID, map[string]any{
			"protocolVersion": negotiateLegacyVersion(init.ProtocolVersion),
			"serverInfo":      serverInfo(version),
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"instructions":    serverInstructions,
		})
	case "notifications/initialized":
		return Response{JSONRPC: "2.0"}
	case "server/discover":
		return errorResponse(req.ID, ErrCodeInvalidParams, "missing _meta "+metaProtocolVersion, nil)
	default:
		return dispatchShared(req, tools)
	}
}

func dispatchModern(req Request, meta map[string]json.RawMessage, rawVersion json.RawMessage, tools *ToolSet, version string) Response {
	var requested string
	if err := json.Unmarshal(rawVersion, &requested); err != nil || requested == "" {
		return errorResponse(req.ID, ErrCodeInvalidParams, "invalid _meta "+metaProtocolVersion, nil)
	}
	if requested != modernProtocolVersion {
		return errorResponse(req.ID, ErrCodeUnsupportedProtocolVersion, "Unsupported protocol version", map[string]any{
			"supported": append([]string{modernProtocolVersion}, legacyProtocolVersions...),
			"requested": requested,
		})
	}
	if _, ok := meta[metaClientCapabilities]; !ok {
		return errorResponse(req.ID, ErrCodeInvalidParams, "missing _meta "+metaClientCapabilities, nil)
	}

	var resp Response
	if req.Method == "server/discover" {
		resp = resultResponse(req.ID, map[string]any{
			"supportedVersions": append([]string{modernProtocolVersion}, legacyProtocolVersions...),
			"capabilities":      map[string]any{"tools": map[string]any{}},
			"instructions":      serverInstructions,
		})
	} else {
		resp = dispatchShared(req, tools)
	}
	if result, ok := resp.Result.(map[string]any); ok {
		result["resultType"] = "complete"
		result["_meta"] = map[string]any{metaServerInfo: serverInfo(version)}
	}
	return resp
}

// dispatchShared handles methods that behave the same in both eras.
func dispatchShared(req Request, tools *ToolSet) Response {
	switch req.Method {
	case "ping":
		return resultResponse(req.ID, map[string]any{})
	case "tools/list":
		return resultResponse(req.ID, map[string]any{"tools": tools.Definitions()})
	case "tools/call":
		return handleToolCall(req, tools)
	default:
		return errorResponse(req.ID, ErrCodeMethodNotFound, fmt.Sprintf("method not found: %s", req.Method), nil)
	}
}

// negotiateLegacyVersion echoes the client's version when supported and
// otherwise offers the newest legacy version, as the legacy lifecycle requires.
func negotiateLegacyVersion(requested string) string {
	for _, v := range legacyProtocolVersions {
		if v == requested {
			return v
		}
	}
	return legacyProtocolVersions[0]
}

func serverInfo(version string) map[string]any {
	return map[string]any{"name": "bifrost", "version": version}
}

func resultResponse(id *json.RawMessage, result map[string]any) Response {
	return Response{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id *json.RawMessage, code int, message string, data any) Response {
	return Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: message, Data: data}}
}

// toolCallParams are the params for tools/call.
type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// handleToolCall reports malformed calls and unknown tools as JSON-RPC
// errors, and failures inside a tool as an isError result the model can read
// and correct.
func handleToolCall(req Request, tools *ToolSet) Response {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error(), nil)
	}
	if params.Name == "" {
		return errorResponse(req.ID, ErrCodeInvalidParams, "invalid params: missing tool name", nil)
	}

	result, err := tools.Call(params.Name, params.Arguments)
	if errors.Is(err, errUnknownTool) {
		return errorResponse(req.ID, ErrCodeInvalidParams, "Unknown tool: "+params.Name, nil)
	}
	if err != nil {
		return resultResponse(req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		})
	}

	content, err := json.Marshal(result)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternalError, "encode tool result: "+err.Error(), nil)
	}
	return resultResponse(req.ID, map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(content)}},
		"isError": false,
	})
}
