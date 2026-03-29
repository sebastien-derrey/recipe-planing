package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// Run starts the MCP stdio server. It blocks until stdin is closed.
func Run(apiBase, mcpToken, anthropicAPIKey string) {
	extractor := NewClaudeExtractor(anthropicAPIKey)
	d := newDispatcher(extractor, apiBase, mcpToken)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		resp := handleMessage(context.Background(), d, line)
		out, _ := json.Marshal(resp)
		fmt.Fprintf(os.Stdout, "%s\n", out)
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin scanner: %v", err)
	}
}

// ── JSON-RPC 2.0 types ────────────────────────────────────────────────────

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func errResp(id any, code int, msg string) response {
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

// ── Message dispatcher ────────────────────────────────────────────────────

func handleMessage(ctx context.Context, d *dispatcher, raw []byte) response {
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		return errResp(nil, -32700, "parse error")
	}

	switch req.Method {
	case "initialize":
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]string{"name": "recipe_manager", "version": "1.0.0"},
				"capabilities":    map[string]any{"tools": map[string]any{}},
			},
		}

	case "initialized":
		// Notification — no response needed but we return an empty one to keep the loop clean
		return response{JSONRPC: "2.0"}

	case "tools/list":
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": tools},
		}

	case "tools/call":
		return handleToolCall(ctx, d, req)

	case "ping":
		return response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}

	default:
		return errResp(req.ID, -32601, "method not found: "+req.Method)
	}
}

func handleToolCall(ctx context.Context, d *dispatcher, req request) response {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errResp(req.ID, -32602, "invalid params")
	}

	result, err := d.dispatch(ctx, params.Name, params.Arguments)
	if err != nil {
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]string{{"type": "text", "text": "Error: " + err.Error()}},
				"isError": true,
			},
		}
	}
	return response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]string{{"type": "text", "text": result}},
		},
	}
}
