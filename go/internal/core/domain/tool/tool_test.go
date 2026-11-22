package tool

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDecodeAppliesDefaultsAndRejectsWithoutEchoingValues(t *testing.T) {
	s := Schema{
		Properties: map[string]Property{
			"service": {Type: "string"},
			"minutes": {Type: "integer", Default: float64(60)},
			"level":   {Type: "string", Enum: []string{"error", "warn"}},
		},
		Required: []string{"service"},
	}
	args, err := s.Decode(json.RawMessage(`{"service":"api"}`))
	if err != nil || Int(args, "minutes", 0) != 60 {
		t.Fatalf("args=%v err=%v", args, err)
	}
	_, err = s.Decode(json.RawMessage(`{"minutes":1.5,"level":"SECRET-VALUE","extra":true}`))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"missing required field service", "field minutes: expected integer", "expected one of error, warn", "unknown field extra"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "SECRET-VALUE") {
		t.Error("validation error echoed a model-sent value")
	}
}
