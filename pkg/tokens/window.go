package tokens

import "ollama_agent_go/pkg/ollama"

// Trim applies a sliding window to fit msgs within budget tokens. It always
// preserves leading system messages and the most recent message, dropping the
// oldest non-system messages first. Tool messages orphaned from their preceding
// assistant turn are also dropped to keep the transcript coherent.
//
// A budget <= 0 disables trimming.
func Trim(msgs []ollama.Message, budget int) []ollama.Message {
	if budget <= 0 || len(msgs) == 0 {
		return msgs
	}

	// Separate leading system messages (always kept).
	sys := 0
	for sys < len(msgs) && msgs[sys].Role == "system" {
		sys++
	}
	system := msgs[:sys]
	rest := msgs[sys:]

	systemCost := EstimateMessages(system)
	avail := budget - systemCost

	// Walk the tail backward, keeping messages until the budget is exhausted.
	kept := make([]ollama.Message, 0, len(rest))
	used := 0
	for i := len(rest) - 1; i >= 0; i-- {
		cost := EstimateMessage(rest[i])
		if used+cost > avail && len(kept) > 0 {
			break
		}
		kept = append(kept, rest[i])
		used += cost
	}

	// kept is reversed; restore order.
	for l, r := 0, len(kept)-1; l < r; l, r = l+1, r-1 {
		kept[l], kept[r] = kept[r], kept[l]
	}

	// Drop a leading orphaned tool message (a tool result with no preceding
	// assistant tool-call turn in the window).
	for len(kept) > 0 && kept[0].Role == "tool" {
		kept = kept[1:]
	}

	out := make([]ollama.Message, 0, len(system)+len(kept))
	out = append(out, system...)
	out = append(out, kept...)
	return out
}
