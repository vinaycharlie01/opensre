// Package evidence models citeable facts gathered by tools during a run.
package evidence

import "fmt"

// Entry is one citeable piece of evidence. Source is the integration key
// (e.g. "datadog"); it is an open string because each integration owns its
// own key and the core must not enumerate vendors.
type Entry struct {
	Source  string `json:"source"`
	Label   string `json:"label"`
	Summary string `json:"summary,omitempty"`
	URL     string `json:"url,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

// Catalog accumulates entries and assigns stable display ids (E1, E2, …) in
// insertion order. The zero value is ready to use. Not safe for concurrent use.
type Catalog struct {
	entries []Entry
}

// Add appends entries.
func (c *Catalog) Add(entries ...Entry) { c.entries = append(c.entries, entries...) }

// Len is the number of entries.
func (c *Catalog) Len() int { return len(c.entries) }

// Entries returns a copy of all entries.
func (c *Catalog) Entries() []Entry {
	out := make([]Entry, len(c.entries))
	copy(out, c.entries)
	return out
}

// Cited is an entry paired with its display id.
type Cited struct {
	ID string `json:"id"`
	Entry
}

// Cited returns every entry with its display id.
func (c *Catalog) Cited() []Cited {
	out := make([]Cited, len(c.entries))
	for i, e := range c.entries {
		out[i] = Cited{ID: fmt.Sprintf("E%d", i+1), Entry: e}
	}
	return out
}
