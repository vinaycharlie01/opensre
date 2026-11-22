package anthropic_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/llm/anthropic"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/conversation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/tool"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

const toolUseResponse = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5",
 "content":[{"type":"thinking","thinking":"","signature":"sig-123"},
            {"type":"text","text":"checking logs"},
            {"type":"tool_use","id":"tu_1","name":"search","input":{"q":"checkout"}}],
 "stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":5}}`

type captured struct {
	beta string
	body map[string]any
}

func server(t *testing.T, status int, reply string, got *[]captured) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		*got = append(*got, captured{beta: r.Header.Get("anthropic-beta"), body: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var searchSpec = tool.Spec{
	Name: "search", Description: "d", Source: "s", SideEffect: tool.ReadOnly,
	InputSchema: tool.Schema{Properties: map[string]tool.Property{"q": {Type: "string"}}, Required: []string{"q"}},
}

func TestRequestShapeAndThinkingReplay(t *testing.T) {
	var got []captured
	srv := server(t, http.StatusOK, toolUseResponse, &got)
	c := anthropic.New(anthropic.Config{APIKey: "test", BaseURL: srv.URL, Effort: "high"})

	history := []conversation.Message{conversation.UserText("investigate")}
	resp, err := c.Complete(context.Background(), ports.LLMRequest{
		System: "sys", Messages: history, Tools: []tool.Spec{searchSpec}, MaxTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StopReason != conversation.StopToolUse || resp.Text != "checking logs" ||
		len(resp.ToolCalls) != 1 || string(resp.ToolCalls[0].Input) != `{"q":"checkout"}` {
		t.Fatalf("bad response mapping: %+v", resp)
	}

	first := got[0]
	if !strings.Contains(first.beta, "server-side-fallback-2026-07-01") || first.body["fallbacks"] != "default" {
		t.Errorf("refusal fallbacks not enabled: beta=%q fallbacks=%v", first.beta, first.body["fallbacks"])
	}
	if first.body["model"] != anthropic.DefaultModel {
		t.Errorf("model = %v", first.body["model"])
	}
	if eff := first.body["output_config"].(map[string]any)["effort"]; eff != "high" {
		t.Errorf("effort = %v", eff)
	}
	schema := first.body["tools"].([]any)[0].(map[string]any)["input_schema"].(map[string]any)
	if schema["required"].([]any)[0] != "q" {
		t.Errorf("schema = %v", schema)
	}

	// Second turn: the assistant turn must be replayed verbatim (thinking
	// signature included) and tool results sent in one user message.
	history = append(history,
		conversation.Message{Role: conversation.RoleAssistant, Text: resp.Text, ToolCalls: resp.ToolCalls, Native: resp.Native},
		conversation.Message{Role: conversation.RoleUser, ToolResults: []conversation.ToolResult{{CallID: "tu_1", Content: "3 errors"}}},
	)
	if _, err := c.Complete(context.Background(), ports.LLMRequest{Messages: history, MaxTokens: 1000}); err != nil {
		t.Fatal(err)
	}
	msgs := got[1].body["messages"].([]any)
	replayed, _ := json.Marshal(msgs[1])
	if !strings.Contains(string(replayed), "sig-123") {
		t.Errorf("thinking block not replayed: %s", replayed)
	}
	results, _ := json.Marshal(msgs[2])
	if !strings.Contains(string(results), `"tool_use_id":"tu_1"`) {
		t.Errorf("tool result missing: %s", results)
	}
}

func TestPromptTooLongMapsToContextOverflow(t *testing.T) {
	var got []captured
	srv := server(t, http.StatusBadRequest,
		`{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long: 1200000 tokens > 1000000 maximum"}}`, &got)
	c := anthropic.New(anthropic.Config{APIKey: "test", BaseURL: srv.URL, DisableFallbacks: true})
	_, err := c.Complete(context.Background(), ports.LLMRequest{Messages: []conversation.Message{conversation.UserText("x")}, MaxTokens: 10})
	if !errors.Is(err, ports.ErrContextOverflow) {
		t.Fatalf("err = %v", err)
	}
	if got[0].beta != "" {
		t.Errorf("fallback beta sent while disabled: %q", got[0].beta)
	}
}
