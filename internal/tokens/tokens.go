// Package tokens provides approximate token counting and sliding-window context
// trimming for conversations sent to a model.
package tokens

import (
	"unicode"

	"ollama_agent_go/internal/types"
)

// Estimator counts tokens in a string.
type Estimator interface {
	Estimate(s string) int
}

// Heuristic estimates tokens by counting word-ish runs and standalone
// punctuation, tracking real BPE counts within ~10-20% for code and prose.
type Heuristic struct{}

// Estimate implements Estimator.
func (Heuristic) Estimate(s string) int {
	count := 0
	inWord := false
	runeCount := 0
	for _, r := range s {
		runeCount++
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if !inWord {
				count++
				inWord = true
			}
		default:
			inWord = false
			if !unicode.IsSpace(r) {
				count++
			}
		}
	}
	if approx := runeCount / 4; approx > count {
		return approx
	}
	return count
}

// DefaultEstimator is used by the package-level helpers.
var DefaultEstimator Estimator = Heuristic{}

// Estimate counts tokens in s using the default estimator.
func Estimate(s string) int { return DefaultEstimator.Estimate(s) }

const messageOverhead = 4

// EstimateMessage counts the tokens for a single message.
func EstimateMessage(m types.Message) int {
	n := messageOverhead + Estimate(m.Content) + Estimate(m.Role) + Estimate(m.ToolName)
	for _, tc := range m.ToolCalls {
		n += Estimate(tc.Function.Name)
		for k, v := range tc.Function.Arguments {
			n += Estimate(k)
			if s, ok := v.(string); ok {
				n += Estimate(s)
			} else {
				n += 2
			}
		}
	}
	return n
}

// EstimateMessages sums the token estimate across messages.
func EstimateMessages(msgs []types.Message) int {
	total := 0
	for _, m := range msgs {
		total += EstimateMessage(m)
	}
	return total
}
