// Package alert models an external alert pushed into OpenSRE for investigation.
package alert

import (
	"errors"
	"strings"
	"time"
)

// MaxTextLength bounds the alert body accepted from an external caller.
const MaxTextLength = 64 * 1024

// Severity is a normalized, lower-case severity label. Vendors use open-ended
// vocabularies, so this is not a closed enum; Normalize only trims and lowers.
type Severity string

// Alert is an incoming alert. Text is the only required field.
type Alert struct {
	Text       string    `json:"text"`
	Name       string    `json:"alert_name,omitempty"`
	Severity   Severity  `json:"severity,omitempty"`
	Source     string    `json:"source,omitempty"`
	ReceivedAt time.Time `json:"received_at"`
}

// Validation errors. Callers at an external boundary should report only that
// validation failed, not echo payload values back.
var (
	ErrEmptyText   = errors.New("alert text is required")
	ErrTextTooLong = errors.New("alert text exceeds maximum length")
)

// Normalize returns a copy with whitespace trimmed and keys lower-cased.
func (a Alert) Normalize() Alert {
	a.Text = strings.TrimSpace(a.Text)
	a.Name = strings.TrimSpace(a.Name)
	a.Severity = Severity(strings.ToLower(strings.TrimSpace(string(a.Severity))))
	a.Source = strings.ToLower(strings.TrimSpace(a.Source))
	return a
}

// Validate reports whether the alert can be investigated.
func (a Alert) Validate() error {
	if strings.TrimSpace(a.Text) == "" {
		return ErrEmptyText
	}
	if len(a.Text) > MaxTextLength {
		return ErrTextTooLong
	}
	return nil
}

// Title is a short human label: the alert name when present, else the first
// line of the text.
func (a Alert) Title() string {
	if a.Name != "" {
		return a.Name
	}
	first, _, _ := strings.Cut(a.Text, "\n")
	const maxTitle = 80
	if len(first) > maxTitle {
		return first[:maxTitle] + "…"
	}
	return first
}
