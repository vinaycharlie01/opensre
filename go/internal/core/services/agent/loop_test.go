package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/llm/scripted"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/conversation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/evidence"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/tool"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
	"github.com/Tracer-Cloud/opensre/go/internal/core/services/agent"
)

type fakeTool struct {
	name     string
	safe     bool
	inflight *int32
	peak     *int32
	invoke   func(args map[string]any) (tool.Output, error)
}

func (f fakeTool) Spec() tool.Spec {
	return tool.Spec{
		Name: f.name, Description: "test tool", Source: "test", SideEffect: tool.ReadOnly,
		ConcurrencySafe: f.safe,
		InputSchema: tool.Schema{
			Properties: map[string]tool.Property{"q": {Type: "string"}},
			Required:   []string{"q"},
		},
	}
}

func (f fakeTool) Invoke(_ context.Context, args map[string]any) (tool.Output, error) {
	if f.inflight != nil {
		n := atomic.AddInt32(f.inflight, 1)
		for {
			p := atomic.LoadInt32(f.peak)
			if n <= p || atomic.CompareAndSwapInt32(f.peak, p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(f.inflight, -1)
	}
	return f.invoke(args)
}

func echo(args map[string]any) (tool.Output, error) {
	return tool.Output{
		Content:  "result for " + tool.String(args, "q"),
		Evidence: []evidence.Entry{{Source: "test", Label: tool.String(args, "q")}},
	}, nil
}

func call(id, name, input string) conversation.ToolCall {
	return conversation.ToolCall{ID: id, Name: name, Input: json.RawMessage(input)}
}

func toolTurn(calls ...conversation.ToolCall) scripted.Step {
	return scripted.Reply(ports.LLMResponse{ToolCalls: calls, StopReason: conversation.StopToolUse})
}

func answer(text string) scripted.Step {
	return scripted.Reply(ports.LLMResponse{Text: text, StopReason: conversation.StopEndTurn})
}

func newRunner(t *testing.T, llm ports.LLM, cfg agent.Config, tools ...ports.Tool) *agent.Runner {
	t.Helper()
	reg, err := agent.NewRegistry(tools...)
	if err != nil {
		t.Fatal(err)
	}
	return agent.NewRunner(llm, reg, cfg)
}

func run(t *testing.T, r *agent.Runner) agent.Result {
	t.Helper()
	res, err := r.Run(context.Background(), agent.Input{Messages: []conversation.Message{conversation.UserText("go")}})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestToolObservationsReachModelAndEvidenceIsCollected(t *testing.T) {
	llm := scripted.New(
		toolTurn(call("c1", "search", `{"q":"checkout"}`)),
		answer("root cause found"),
	)
	res := run(t, newRunner(t, llm, agent.DefaultConfig(), fakeTool{name: "search", invoke: echo}))

	if res.Stop != agent.StopAnswered || res.Text != "root cause found" || res.ToolCalls != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Evidence.Len() != 1 || res.Evidence.Cited()[0].ID != "E1" {
		t.Fatalf("evidence not collected: %+v", res.Evidence.Entries())
	}
	second := llm.Requests[1].Messages
	last := second[len(second)-1]
	if len(last.ToolResults) != 1 || last.ToolResults[0].Content != "result for checkout" || last.ToolResults[0].CallID != "c1" {
		t.Fatalf("tool result not fed back: %+v", last)
	}
}

func TestBadCallsBecomeErrorObservationsNotRunFailures(t *testing.T) {
	llm := scripted.New(
		toolTurn(
			call("a", "missing_tool", `{}`),
			call("b", "search", `{"q": 5}`),
			call("c", "search", `{}`),
			call("d", "boom", `{"q":"x"}`),
			call("e", "panics", `{"q":"x"}`),
		),
		answer("done"),
	)
	r := newRunner(t, llm, agent.DefaultConfig(),
		fakeTool{name: "search", invoke: echo},
		fakeTool{name: "boom", invoke: func(map[string]any) (tool.Output, error) {
			return tool.Output{}, errors.New("upstream said token=supersecretvalue")
		}},
		fakeTool{name: "panics", invoke: func(map[string]any) (tool.Output, error) { panic("bad") }},
	)
	res := run(t, r)
	results := llm.Requests[1].Messages[2].ToolResults
	want := []string{"unknown tool", "field q: expected string", "missing required field q", "boom failed", "panics crashed"}
	for i, w := range want {
		if !results[i].IsError || !strings.Contains(results[i].Content, w) {
			t.Errorf("result %d = %+v, want error containing %q", i, results[i], w)
		}
	}
	if strings.Contains(results[3].Content, "supersecretvalue") {
		t.Errorf("secret leaked to model: %q", results[3].Content)
	}
	if res.Stop != agent.StopAnswered {
		t.Fatalf("stop = %s", res.Stop)
	}
}

func TestRepeatedIdenticalBatchesStopWithHandoff(t *testing.T) {
	same := toolTurn(call("x", "search", `{"q":"same"}`))
	var handoffReq ports.LLMRequest
	llm := scripted.New(same, same, same, func(req ports.LLMRequest) (ports.LLMResponse, error) {
		handoffReq = req
		return ports.LLMResponse{Text: "partial summary", StopReason: conversation.StopEndTurn}, nil
	})
	res := run(t, newRunner(t, llm, agent.DefaultConfig(), fakeTool{name: "search", invoke: echo}))

	if res.Stop != agent.StopStagnated || res.Text != "partial summary" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(handoffReq.Tools) != 0 {
		t.Fatal("handoff must disable tools")
	}
}

func TestIterationCapFallsBackWhenHandoffFails(t *testing.T) {
	cfg := agent.DefaultConfig()
	cfg.MaxIterations = 2
	llm := scripted.New(
		toolTurn(call("1", "search", `{"q":"a"}`)),
		toolTurn(call("2", "search", `{"q":"b"}`)),
		// No step left: the handoff call fails and the fixed fallback is used.
	)
	res := run(t, newRunner(t, llm, cfg, fakeTool{name: "search", invoke: echo}))
	if res.Stop != agent.StopIterationCap || !strings.Contains(res.Text, "iteration ceiling") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestContextOverflowShrinksBudgetAndRetries(t *testing.T) {
	big := strings.Repeat("x", 5000)
	llm := scripted.New(
		toolTurn(call("1", "search", `{"q":"a"}`)),
		toolTurn(call("2", "search", `{"q":"b"}`)),
		func(ports.LLMRequest) (ports.LLMResponse, error) {
			return ports.LLMResponse{}, ports.ErrContextOverflow
		},
		answer("fits now"),
	)
	r := newRunner(t, llm, agent.DefaultConfig(), fakeTool{name: "search", invoke: func(map[string]any) (tool.Output, error) {
		return tool.Output{Content: big}, nil
	}})
	res := run(t, r)
	if res.Text != "fits now" {
		t.Fatalf("text = %q", res.Text)
	}
	retry := llm.Requests[3].Messages
	if !strings.HasPrefix(retry[2].ToolResults[0].Content, "[elided") {
		t.Fatal("oldest tool result should be elided on retry")
	}
	if retry[4].ToolResults[0].Content != big {
		t.Fatal("latest tool batch must be kept intact")
	}
}

func TestOnlyConcurrencySafeBatchesRunInParallel(t *testing.T) {
	for _, safe := range []bool{true, false} {
		var inflight, peak int32
		llm := scripted.New(
			toolTurn(call("1", "t", `{"q":"a"}`), call("2", "t", `{"q":"b"}`), call("3", "t", `{"q":"c"}`)),
			answer("ok"),
		)
		run(t, newRunner(t, llm, agent.DefaultConfig(),
			fakeTool{name: "t", safe: safe, inflight: &inflight, peak: &peak, invoke: echo}))
		if safe && peak < 2 {
			t.Errorf("safe tools ran serially (peak %d)", peak)
		}
		if !safe && peak != 1 {
			t.Errorf("unsafe tools overlapped (peak %d)", peak)
		}
	}
}
