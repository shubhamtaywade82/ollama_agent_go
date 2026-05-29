package governance

import (
	"strings"
	"testing"
)

func TestDetectEmail(t *testing.T) {
	matches := Detect("Contact me at alice@example.com for details.")
	found := false
	for _, m := range matches {
		if m.Kind == PIIEmail && m.Value == "alice@example.com" {
			found = true
		}
	}
	if !found {
		t.Error("expected email match not found")
	}
}

func TestDetectPhone(t *testing.T) {
	matches := Detect("Call 555-867-5309 for info.")
	found := false
	for _, m := range matches {
		if m.Kind == PIIPhone {
			found = true
		}
	}
	if !found {
		t.Error("expected phone match not found")
	}
}

func TestDetectSSN(t *testing.T) {
	matches := Detect("SSN: 123-45-6789")
	found := false
	for _, m := range matches {
		if m.Kind == PIISSN {
			found = true
		}
	}
	if !found {
		t.Error("expected SSN match not found")
	}
}

func TestScrubReplacesAll(t *testing.T) {
	text := "Email alice@example.com SSN 123-45-6789"
	scrubbed := Scrub(text)
	if strings.Contains(scrubbed, "alice@example.com") {
		t.Error("email not scrubbed")
	}
	if strings.Contains(scrubbed, "123-45-6789") {
		t.Error("SSN not scrubbed")
	}
	if !strings.Contains(scrubbed, "[REDACTED:") {
		t.Error("expected [REDACTED:...] placeholder")
	}
}

func TestDetectNegative(t *testing.T) {
	matches := Detect("This is a plain sentence with no personal information.")
	if len(matches) != 0 {
		t.Errorf("expected no matches, got %d: %+v", len(matches), matches)
	}
}
