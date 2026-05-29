package governance

import (
	"fmt"
	"regexp"
)

// SecretPattern pairs a name with a compiled regex for detecting a secret type.
type SecretPattern struct {
	Name    string
	Pattern *regexp.Regexp
}

// DefaultPatterns covers common secret formats: AWS keys, GitHub tokens, OpenAI/Anthropic
// keys, generic "key = ..." assignments, and bearer tokens.
var DefaultPatterns = []SecretPattern{
	{"aws_access_key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"aws_secret_key", regexp.MustCompile(`\b[0-9a-zA-Z/+]{40}\b`)},
	{"github_token", regexp.MustCompile(`\b(ghp_|gho_|ghu_|ghs_|ghr_)[A-Za-z0-9]{36,}\b`)},
	{"openai_key", regexp.MustCompile(`\bsk-[A-Za-z0-9]{48,}\b`)},
	{"anthropic_key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9\-]{50,}\b`)},
	{"bearer_token", regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9\-._~+/]{20,}\b`)},
	{"generic_api_key", regexp.MustCompile(`(?i)(api[_\s-]?key|secret[_\s-]?key|access[_\s-]?token)\s*[=:]\s*["']?[A-Za-z0-9\-._~+/]{16,}["']?`)},
}

// ScanMessage returns true if text contains a likely secret according to patterns.
func ScanMessage(text string, patterns []SecretPattern) bool {
	for _, p := range patterns {
		if p.Pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// RedactSecrets replaces each matched secret with [SECRET:<name>].
func RedactSecrets(text string, patterns []SecretPattern) string {
	for _, p := range patterns {
		text = p.Pattern.ReplaceAllStringFunc(text, func(_ string) string {
			return fmt.Sprintf("[SECRET:%s]", p.Name)
		})
	}
	return text
}
