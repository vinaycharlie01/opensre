// Package anthropic adapts the Claude Messages API to the ports.LLM port using
// the official Anthropic Go SDK.
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/conversation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// DefaultModel is used when no model is configured.
const DefaultModel = "claude-opus-5-5"

// fallbackBeta enables server-side refusal fallbacks; with fallbacks
// "default" the API re-runs a declined request on a suitable model.
const fallbackBeta = "server-side-fallback-2026-07-01"

// Config configures the adapter.
type Config struct {
	// APIKey is optional: when empty the SDK resolves credentials from the
	// environment (ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN, or an `ant` profile).
	APIKey string
	Model  string
	// Effort is low|medium|high|xhigh|max; empty uses the model default.
	Effort string
	// DisableFallbacks turns off server-side refusal fallbacks.
	DisableFallbacks bool
	// BaseURL overrides the API endpoint (tests, proxies).
	BaseURL string
}

// Client implements ports.LLM.
type Client struct {
	sdk sdk.Client
	cfg Config
}

var _ ports.LLM = (*Client)(nil)

// New builds a client.
func New(cfg Config) *Client {
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	var opts []option.RequestOption
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	return &Client{sdk: sdk.NewClient(opts...), cfg: cfg}
}

// Complete implements ports.LLM.
func (c *Client) Complete(ctx context.Context, req ports.LLMRequest) (ports.LLMResponse, error) {
	params := sdk.MessageNewParams{
		Model:     sdk.Model(c.cfg.Model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  toMessages(req.Messages),
		Tools:     toTools(req),
	}
	if req.System != "" {
		params.System = []sdk.TextBlockParam{{Text: req.System}}
	}
	if c.cfg.Effort != "" {
		params.OutputConfig = sdk.OutputConfigParam{Effort: sdk.OutputConfigEffort(c.cfg.Effort)}
	}
	var opts []option.RequestOption
	if !c.cfg.DisableFallbacks {
		opts = append(opts,
			option.WithHeaderAdd("anthropic-beta", fallbackBeta),
			option.WithJSONSet("fallbacks", "default"))
	}

	msg, err := c.sdk.Messages.New(ctx, params, opts...)
	if err != nil {
		return ports.LLMResponse{}, classify(err)
	}
	return fromMessage(msg), nil
}

func toTools(req ports.LLMRequest) []sdk.ToolUnionParam {
	out := make([]sdk.ToolUnionParam, 0, len(req.Tools))
	for _, spec := range req.Tools {
		props := map[string]any{}
		for name, p := range spec.InputSchema.Properties {
			props[name] = p
		}
		t := sdk.ToolParam{
			Name:        spec.Name,
			Description: sdk.String(spec.Description),
			InputSchema: sdk.ToolInputSchemaParam{
				Properties: props,
				Required:   spec.InputSchema.Required,
			},
		}
		out = append(out, sdk.ToolUnionParam{OfTool: &t})
	}
	return out
}

func toMessages(msgs []conversation.Message) []sdk.MessageParam {
	out := make([]sdk.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case conversation.RoleAssistant:
			if native, ok := m.Native.(sdk.MessageParam); ok {
				out = append(out, native) // replays thinking blocks unchanged
				continue
			}
			var blocks []sdk.ContentBlockParamUnion
			if m.Text != "" {
				blocks = append(blocks, sdk.NewTextBlock(m.Text))
			}
			for _, call := range m.ToolCalls {
				blocks = append(blocks, sdk.NewToolUseBlock(call.ID, json.RawMessage(call.Input), call.Name))
			}
			out = append(out, sdk.NewAssistantMessage(blocks...))
		default:
			var blocks []sdk.ContentBlockParamUnion
			// All results of a batch travel in one user message.
			for _, r := range m.ToolResults {
				blocks = append(blocks, sdk.NewToolResultBlock(r.CallID, r.Content, r.IsError))
			}
			if m.Text != "" {
				blocks = append(blocks, sdk.NewTextBlock(m.Text))
			}
			out = append(out, sdk.NewUserMessage(blocks...))
		}
	}
	return out
}

func fromMessage(msg *sdk.Message) ports.LLMResponse {
	resp := ports.LLMResponse{
		StopReason: stopReason(string(msg.StopReason)),
		Usage: conversation.Usage{
			InputTokens:     int(msg.Usage.InputTokens),
			OutputTokens:    int(msg.Usage.OutputTokens),
			CacheReadTokens: int(msg.Usage.CacheReadInputTokens),
		},
		Native: msg.ToParam(),
	}
	var text []string
	for _, block := range msg.Content {
		switch v := block.AsAny().(type) {
		case sdk.TextBlock:
			text = append(text, v.Text)
		case sdk.ToolUseBlock:
			resp.ToolCalls = append(resp.ToolCalls, conversation.ToolCall{
				ID: v.ID, Name: v.Name, Input: json.RawMessage(v.JSON.Input.Raw()),
			})
		}
	}
	resp.Text = strings.Join(text, "\n")
	return resp
}

func stopReason(s string) conversation.StopReason {
	switch s {
	case "end_turn", "stop_sequence":
		return conversation.StopEndTurn
	case "tool_use":
		return conversation.StopToolUse
	case "max_tokens":
		return conversation.StopMaxTokens
	case "refusal":
		return conversation.StopRefusal
	default:
		return conversation.StopOther
	}
}

// classify maps "prompt is too long" rejections onto ErrContextOverflow.
func classify(err error) error {
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest &&
		strings.Contains(strings.ToLower(apiErr.Error()), "prompt is too long") {
		return errors.Join(ports.ErrContextOverflow, err)
	}
	return err
}
