// Package agent is the provider-agnostic ReAct loop: ask the model what to do,
// run the tools it requests, feed the observations back, and repeat until it
// answers without tools or a safety limit stops the run.
package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/conversation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/evidence"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/masking"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/tool"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Stop says why a run ended.
type Stop string

// Run endings. Only StopAnswered means the model finished on its own.
const (
	StopAnswered     Stop = "answered"
	StopIterationCap Stop = "iteration_cap"
	StopStagnated    Stop = "stagnated"
	StopModel        Stop = "model_stopped"
)

// Config bounds one run.
type Config struct {
	// MaxIterations caps model turns that request tools.
	MaxIterations int
	// MaxStagnantBatches stops the run after this many consecutive tool
	// batches that repeat the previous batch's inputs and observations.
	MaxStagnantBatches int
	// MaxTokens is the per-call output cap passed to the model.
	MaxTokens int
	// ContextBudgetChars is the transcript size above which old tool results
	// are elided. Zero disables budgeting.
	ContextBudgetChars int
	// ToolTimeout bounds each tool invocation.
	ToolTimeout time.Duration
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		MaxIterations:      25,
		MaxStagnantBatches: 2,
		MaxTokens:          16000,
		ContextBudgetChars: 400_000,
		ToolTimeout:        60 * time.Second,
	}
}

// Input is one run's task.
type Input struct {
	System   string
	Messages []conversation.Message
}

// Result is a finished run.
type Result struct {
	Text       string
	Stop       Stop
	Transcript []conversation.Message
	Evidence   evidence.Catalog
	ToolCalls  int
	Usage      conversation.Usage
}

// Runner drives the loop. It is safe for concurrent use: all per-run state
// lives on the stack of Run.
type Runner struct {
	llm    ports.LLM
	tools  *Registry
	events ports.EventSink
	mask   masking.Masker
	cfg    Config
}

// Option configures a Runner.
type Option func(*Runner)

// WithEvents sets the event sink.
func WithEvents(s ports.EventSink) Option { return func(r *Runner) { r.events = s } }

// WithMasker sets the masker applied to tool output before the model sees it.
func WithMasker(m masking.Masker) Option { return func(r *Runner) { r.mask = m } }

// NewRunner builds a Runner.
func NewRunner(llm ports.LLM, tools *Registry, cfg Config, opts ...Option) *Runner {
	r := &Runner{llm: llm, tools: tools, cfg: cfg, events: noopSink{}, mask: masking.Default()}
	for _, o := range opts {
		o(r)
	}
	return r
}

const handoffPrompt = `The tool loop has stopped for safety (%s). Tools are disabled for this response.
Give a concise final handoff based only on the work and tool results above:
summarize useful partial results, state what prevented completion, and give the next
practical step. Do not claim the task completed and do not request another tool call.`

var fallbackText = map[Stop]string{
	StopIterationCap: "I reached the tool-iteration ceiling. Partial results are preserved, but I could not complete the request. Continue with a narrower next step.",
	StopStagnated:    "I stopped after repeated tool calls produced no new result. Change the inputs or tool strategy before continuing.",
}

// overflowShrink is the share of the rejected size the budget drops to after a
// provider rejects a request as too large.
const overflowShrink = 0.6

// Run executes the loop to completion.
func (r *Runner) Run(ctx context.Context, in Input) (Result, error) {
	res := Result{Transcript: append([]conversation.Message(nil), in.Messages...)}
	specs := r.tools.Specs()
	budget := r.cfg.ContextBudgetChars
	var lastPrint []byte
	stagnant := 0

	for iter := 1; ; iter++ {
		if iter > r.cfg.MaxIterations {
			return r.handoff(ctx, in.System, res, StopIterationCap)
		}
		r.events.Emit(ctx, ports.Event{Kind: ports.EventTurnStart, Iteration: iter})

		resp, err := r.complete(ctx, ports.LLMRequest{
			System: in.System, Messages: res.Transcript, Tools: specs, MaxTokens: r.cfg.MaxTokens,
		}, &budget)
		if err != nil {
			return res, err
		}
		res.Usage.Add(resp.Usage)
		res.Transcript = append(res.Transcript, conversation.Message{
			Role: conversation.RoleAssistant, Text: resp.Text, ToolCalls: resp.ToolCalls, Native: resp.Native,
		})

		if len(resp.ToolCalls) == 0 {
			res.Text = resp.Text
			res.Stop = StopAnswered
			if resp.StopReason != conversation.StopEndTurn && resp.StopReason != conversation.StopToolUse {
				res.Stop = StopModel
			}
			r.events.Emit(ctx, ports.Event{Kind: ports.EventAnswer, Iteration: iter, Text: resp.Text})
			return res, nil
		}

		results, ev := r.execute(ctx, iter, resp.ToolCalls)
		res.ToolCalls += len(resp.ToolCalls)
		res.Evidence.Add(ev...)
		res.Transcript = append(res.Transcript, conversation.Message{
			Role: conversation.RoleUser, ToolResults: results,
		})

		fp := fingerprint(resp.ToolCalls, results)
		if lastPrint != nil && bytes.Equal(fp, lastPrint) {
			stagnant++
			if stagnant >= r.cfg.MaxStagnantBatches {
				return r.handoff(ctx, in.System, res, StopStagnated)
			}
		} else {
			stagnant = 0
		}
		lastPrint = fp
	}
}

// complete calls the model within the context budget, shrinking the budget and
// retrying when the provider reports an overflow.
func (r *Runner) complete(ctx context.Context, req ports.LLMRequest, budget *int) (ports.LLMResponse, error) {
	const maxOverflowRetries = 2
	full := req.Messages
	for attempt := 0; ; attempt++ {
		req.Messages = fitBudget(full, *budget)
		resp, err := r.llm.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		if !errors.Is(err, ports.ErrContextOverflow) || attempt >= maxOverflowRetries {
			return ports.LLMResponse{}, fmt.Errorf("llm call: %w", err)
		}
		*budget = int(float64(totalSize(req.Messages)) * overflowShrink)
	}
}

// handoff asks the model, with tools disabled, for a final partial summary.
func (r *Runner) handoff(ctx context.Context, system string, res Result, why Stop) (Result, error) {
	r.events.Emit(ctx, ports.Event{Kind: ports.EventHandoff, Text: string(why)})
	res.Stop = why
	msgs := append(append([]conversation.Message(nil), res.Transcript...),
		conversation.UserText(fmt.Sprintf(handoffPrompt, why)))
	budget := r.cfg.ContextBudgetChars
	resp, err := r.complete(ctx, ports.LLMRequest{System: system, Messages: msgs, MaxTokens: r.cfg.MaxTokens}, &budget)
	if err != nil || resp.Text == "" {
		res.Text = fallbackText[why]
		return res, nil
	}
	res.Usage.Add(resp.Usage)
	res.Text = resp.Text
	return res, nil
}

// execute runs one batch. Calls run in parallel only when every tool in the
// batch is concurrency-safe; results keep call order either way.
func (r *Runner) execute(ctx context.Context, iter int, calls []conversation.ToolCall) ([]conversation.ToolResult, []evidence.Entry) {
	results := make([]conversation.ToolResult, len(calls))
	evs := make([][]evidence.Entry, len(calls))
	parallel := len(calls) > 1
	for _, c := range calls {
		if t, ok := r.tools.Lookup(c.Name); !ok || !t.Spec().ConcurrencySafe {
			parallel = false
			break
		}
	}
	run := func(i int) {
		results[i], evs[i] = r.invoke(ctx, iter, calls[i])
	}
	if parallel {
		var wg sync.WaitGroup
		for i := range calls {
			wg.Add(1)
			go func(i int) { defer wg.Done(); run(i) }(i)
		}
		wg.Wait()
	} else {
		for i := range calls {
			run(i)
		}
	}
	var all []evidence.Entry
	for _, e := range evs {
		all = append(all, e...)
	}
	return results, all
}

func (r *Runner) invoke(ctx context.Context, iter int, call conversation.ToolCall) (res conversation.ToolResult, ev []evidence.Entry) {
	start := time.Now()
	res.CallID = call.ID
	r.events.Emit(ctx, ports.Event{Kind: ports.EventToolStart, Iteration: iter, ToolName: call.Name})
	defer func() {
		if p := recover(); p != nil {
			res.Content, res.IsError, ev = fmt.Sprintf("tool %s crashed", call.Name), true, nil
		}
		r.events.Emit(ctx, ports.Event{
			Kind: ports.EventToolEnd, Iteration: iter, ToolName: call.Name,
			IsError: res.IsError, Duration: time.Since(start),
		})
	}()

	t, ok := r.tools.Lookup(call.Name)
	if !ok {
		res.Content, res.IsError = fmt.Sprintf("unknown tool %q", call.Name), true
		return res, nil
	}
	args, err := t.Spec().InputSchema.Decode(call.Input)
	if err != nil {
		res.Content, res.IsError = err.Error(), true
		return res, nil
	}
	tctx, cancel := context.WithTimeout(ctx, r.cfg.ToolTimeout)
	defer cancel()
	out, err := t.Invoke(tctx, args)
	if err != nil {
		res.Content, res.IsError = r.mask.Mask(fmt.Sprintf("%s failed: %v", call.Name, err)), true
		return res, nil
	}
	res.Content = r.mask.Mask(out.Content)
	for i := range out.Evidence {
		out.Evidence[i].Snippet = r.mask.Mask(out.Evidence[i].Snippet)
		out.Evidence[i].Summary = r.mask.Mask(out.Evidence[i].Summary)
	}
	return res, out.Evidence
}

// fingerprint identifies a batch by tool names, canonical inputs, and
// observations. A failed call's message is excluded: repeating a failing call
// with identical input is not progress even if each error carries a fresh
// request id.
func fingerprint(calls []conversation.ToolCall, results []conversation.ToolResult) []byte {
	h := sha256.New()
	for i, c := range calls {
		fmt.Fprintf(h, "%s\x00%s\x00%t\x00", c.Name, canonicalJSON(c.Input), results[i].IsError)
		if !results[i].IsError {
			h.Write([]byte(results[i].Content))
		}
		h.Write([]byte{0})
	}
	return h.Sum(nil)
}

func canonicalJSON(raw json.RawMessage) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	b, _ := json.Marshal(v) // map keys are emitted sorted
	return string(b)
}

type noopSink struct{}

func (noopSink) Emit(context.Context, ports.Event) {}

// Specs exposes the registry's specs (ToolCatalog driving port).
func (r *Runner) Specs() []tool.Spec { return r.tools.Specs() }
