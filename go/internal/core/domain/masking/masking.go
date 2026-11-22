// Package masking redacts secrets from text before it leaves the process
// (to an LLM or an external response). Pure and deterministic.
package masking

import "regexp"

type rule struct {
	re          *regexp.Regexp
	replacement string
}

// Masker applies an ordered list of redaction rules. The zero value redacts
// nothing; use Default for the built-in rules.
type Masker struct {
	rules []rule
}

// Default returns a Masker with the built-in secret rules.
func Default() Masker {
	return Masker{rules: []rule{
		{regexp.MustCompile(`(?i)\b(bearer)\s+[A-Za-z0-9._~+/=-]{8,}`), "$1 [REDACTED]"},
		{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "[REDACTED_AWS_KEY]"},
		{regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{8,}`), "[REDACTED_API_KEY]"},
		{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`), "[REDACTED_GITHUB_TOKEN]"},
		{regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`), "[REDACTED_SLACK_TOKEN]"},
		{regexp.MustCompile(`(?i)\b(api[_-]?key|app[_-]?key|password|passwd|secret|token)(["']?\s*[:=]\s*["']?)[^\s"',}]{4,}`), "$1$2[REDACTED]"},
	}}
}

// WithPattern returns a copy with an extra rule appended.
func (m Masker) WithPattern(re *regexp.Regexp, replacement string) Masker {
	rules := make([]rule, len(m.rules), len(m.rules)+1)
	copy(rules, m.rules)
	return Masker{rules: append(rules, rule{re, replacement})}
}

// Mask returns text with every rule applied in order.
func (m Masker) Mask(text string) string {
	for _, r := range m.rules {
		text = r.re.ReplaceAllString(text, r.replacement)
	}
	return text
}
