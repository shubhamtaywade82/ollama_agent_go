package governance

import (
	"context"
	"regexp"
)

// GuardrailResult is the outcome of a guardrail check.
type GuardrailResult struct {
	Blocked bool
	Reason  string
}

// Guardrail can inspect a message before it reaches the LLM (CheckInput) and
// after the LLM responds (CheckOutput).
type Guardrail interface {
	CheckInput(ctx context.Context, input string) GuardrailResult
	CheckOutput(ctx context.Context, output string) GuardrailResult
}

// KeywordGuardrail blocks text matching any of the configured regexp patterns.
type KeywordGuardrail struct {
	BlockedInputPatterns  []*regexp.Regexp
	BlockedOutputPatterns []*regexp.Regexp
}

// NewKeywordGuardrail compiles the provided pattern strings into a KeywordGuardrail.
// Returns an error if any pattern fails to compile.
func NewKeywordGuardrail(inputPatterns, outputPatterns []string) (*KeywordGuardrail, error) {
	inp, err := compilePatterns(inputPatterns)
	if err != nil {
		return nil, err
	}
	out, err := compilePatterns(outputPatterns)
	if err != nil {
		return nil, err
	}
	return &KeywordGuardrail{BlockedInputPatterns: inp, BlockedOutputPatterns: out}, nil
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	res := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		res = append(res, re)
	}
	return res, nil
}

// CheckInput returns a blocked result if the input matches any blocked input pattern.
func (g *KeywordGuardrail) CheckInput(_ context.Context, input string) GuardrailResult {
	for _, re := range g.BlockedInputPatterns {
		if re.MatchString(input) {
			return GuardrailResult{Blocked: true, Reason: "input matches blocked pattern: " + re.String()}
		}
	}
	return GuardrailResult{}
}

// CheckOutput returns a blocked result if the output matches any blocked output pattern.
func (g *KeywordGuardrail) CheckOutput(_ context.Context, output string) GuardrailResult {
	for _, re := range g.BlockedOutputPatterns {
		if re.MatchString(output) {
			return GuardrailResult{Blocked: true, Reason: "output matches blocked pattern: " + re.String()}
		}
	}
	return GuardrailResult{}
}

// NoopGuardrail allows everything.
type NoopGuardrail struct{}

func (NoopGuardrail) CheckInput(_ context.Context, _ string) GuardrailResult  { return GuardrailResult{} }
func (NoopGuardrail) CheckOutput(_ context.Context, _ string) GuardrailResult { return GuardrailResult{} }
