package governance

import (
	"fmt"
	"regexp"
)

// PIIKind identifies the category of personal information detected.
type PIIKind string

const (
	PIIEmail      PIIKind = "email"
	PIIPhone      PIIKind = "phone"
	PIISSN        PIIKind = "ssn"
	PIICreditCard PIIKind = "credit_card"
	PIIIPAddress  PIIKind = "ip_address"
)

// PIIMatch is one detected PII span within a text.
type PIIMatch struct {
	Kind  PIIKind
	Start int
	End   int
	Value string
}

type piiPattern struct {
	kind    PIIKind
	re      *regexp.Regexp
}

var piiPatterns = []piiPattern{
	{PIIEmail, regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)},
	{PIIPhone, regexp.MustCompile(`\b(\+?1[-.\s]?)?(\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4})\b`)},
	{PIISSN, regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)},
	{PIICreditCard, regexp.MustCompile(`\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13}|3(?:0[0-5]|[68][0-9])[0-9]{11}|6(?:011|5[0-9]{2})[0-9]{12})\b`)},
	{PIIIPAddress, regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)},
}

// Detect returns all PII matches found in text.
func Detect(text string) []PIIMatch {
	var matches []PIIMatch
	for _, p := range piiPatterns {
		locs := p.re.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			matches = append(matches, PIIMatch{
				Kind:  p.kind,
				Start: loc[0],
				End:   loc[1],
				Value: text[loc[0]:loc[1]],
			})
		}
	}
	return matches
}

// Scrub replaces all PII matches in text with [REDACTED:<kind>].
func Scrub(text string) string {
	for _, p := range piiPatterns {
		text = p.re.ReplaceAllStringFunc(text, func(m string) string {
			return fmt.Sprintf("[REDACTED:%s]", p.kind)
		})
	}
	return text
}
