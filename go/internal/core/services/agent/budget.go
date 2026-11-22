package agent

import (
	"fmt"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/conversation"
)

// fitBudget returns messages whose total size is within budget characters by
// replacing the content of the oldest tool results with a short placeholder.
// The first message (the task) and the most recent tool batch are never
// elided, so the model always sees what it was asked and its last observation.
// The input slice is not modified.
func fitBudget(msgs []conversation.Message, budget int) []conversation.Message {
	if budget <= 0 || totalSize(msgs) <= budget {
		return msgs
	}
	out := make([]conversation.Message, len(msgs))
	copy(out, msgs)
	total := totalSize(out)
	lastBatch := -1
	for i := len(out) - 1; i >= 0; i-- {
		if len(out[i].ToolResults) > 0 {
			lastBatch = i
			break
		}
	}
	for i := 1; i < len(out) && total > budget; i++ {
		if i == lastBatch || len(out[i].ToolResults) == 0 {
			continue
		}
		results := make([]conversation.ToolResult, len(out[i].ToolResults))
		copy(results, out[i].ToolResults)
		for j := range results {
			before := len(results[j].Content)
			placeholder := fmt.Sprintf("[elided %d chars to fit the context budget]", before)
			if before <= len(placeholder) {
				continue
			}
			results[j].Content = placeholder
			total -= before - len(placeholder)
		}
		out[i].ToolResults = results
	}
	return out
}

func totalSize(msgs []conversation.Message) int {
	n := 0
	for _, m := range msgs {
		n += m.Size()
	}
	return n
}
