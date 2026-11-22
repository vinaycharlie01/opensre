// Package tool defines the contract for agent-callable tools: their metadata,
// input schema validation, and output shape. Implementations live in adapters.
package tool

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/evidence"
)

// SideEffect classifies what a tool may change in the outside world.
type SideEffect string

// Side-effect levels, from safest to most dangerous.
const (
	ReadOnly    SideEffect = "read_only"
	Mutating    SideEffect = "mutating"
	Destructive SideEffect = "destructive"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Spec is a tool's static metadata, as advertised to the model.
type Spec struct {
	Name        string
	Description string
	// Source is the owning integration key, e.g. "grafana".
	Source     string
	SideEffect SideEffect
	// ConcurrencySafe tools may run in parallel with other safe tools.
	ConcurrencySafe bool
	// InputSchema is a JSON Schema object ({"type":"object","properties":…}).
	InputSchema Schema
}

// Validate checks the spec is well formed.
func (s Spec) Validate() error {
	if !namePattern.MatchString(s.Name) {
		return fmt.Errorf("tool name %q must match %s", s.Name, namePattern)
	}
	if strings.TrimSpace(s.Description) == "" {
		return fmt.Errorf("tool %q needs a description", s.Name)
	}
	if s.Source == "" {
		return fmt.Errorf("tool %q needs a source", s.Name)
	}
	switch s.SideEffect {
	case ReadOnly, Mutating, Destructive:
	default:
		return fmt.Errorf("tool %q has unknown side effect %q", s.Name, s.SideEffect)
	}
	return nil
}

// Output is what a tool returns. Content is what the model sees; Evidence is
// recorded in the run's catalog so the final report can cite it.
type Output struct {
	Content  string
	Evidence []evidence.Entry
}

// JSONOutput renders v as indented JSON content.
func JSONOutput(v any, ev ...evidence.Entry) (Output, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return Output{}, err
	}
	return Output{Content: string(b), Evidence: ev}, nil
}

// Property describes one input field.
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// Schema is the subset of JSON Schema the tool layer validates: a flat object
// of typed properties. Richer nesting is passed to the model but not checked.
type Schema struct {
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// MarshalJSON emits a full JSON Schema object.
func (s Schema) MarshalJSON() ([]byte, error) {
	props := s.Properties
	if props == nil {
		props = map[string]Property{}
	}
	req := s.Required
	if req == nil {
		req = []string{}
	}
	return json.Marshal(map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             req,
		"additionalProperties": false,
	})
}

// ErrInvalidInput wraps every validation failure.
var ErrInvalidInput = errors.New("invalid tool input")

// Decode parses raw input, applies defaults, and validates it. Error messages
// name fields and expected types only, never the values the model sent.
func (s Schema) Decode(raw json.RawMessage) (map[string]any, error) {
	args := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("%w: input must be a JSON object", ErrInvalidInput)
		}
	}
	var problems []string
	for _, name := range s.Required {
		if _, ok := args[name]; !ok {
			problems = append(problems, "missing required field "+name)
		}
	}
	for name := range args {
		if _, ok := s.Properties[name]; !ok {
			problems = append(problems, "unknown field "+name)
		}
	}
	for name, prop := range s.Properties {
		v, ok := args[name]
		if !ok {
			if prop.Default != nil {
				args[name] = prop.Default
			}
			continue
		}
		if !typeMatches(v, prop.Type) {
			problems = append(problems, fmt.Sprintf("field %s: expected %s", name, prop.Type))
			continue
		}
		if len(prop.Enum) > 0 {
			sv, _ := v.(string)
			if !contains(prop.Enum, sv) {
				problems = append(problems, fmt.Sprintf("field %s: expected one of %s", name, strings.Join(prop.Enum, ", ")))
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("%w: %s", ErrInvalidInput, strings.Join(problems, "; "))
	}
	return args, nil
}

func typeMatches(v any, typ string) bool {
	switch typ {
	case "string":
		_, ok := v.(string)
		return ok
	case "integer":
		f, ok := v.(float64)
		return ok && f == float64(int64(f))
	case "number":
		_, ok := v.(float64)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	default:
		return true
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// String reads a string argument, or "" when absent.
func String(args map[string]any, key string) string {
	s, _ := args[key].(string)
	return s
}

// Int reads an integer argument, or def when absent.
func Int(args map[string]any, key string, def int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return def
	}
}
