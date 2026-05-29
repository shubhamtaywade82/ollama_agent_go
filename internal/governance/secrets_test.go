package governance

import (
	"strings"
	"testing"
)

func TestScanAWSKey(t *testing.T) {
	text := "Export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
	if !ScanMessage(text, DefaultPatterns) {
		t.Error("expected AWS access key to be detected")
	}
}

func TestScanGitHubToken(t *testing.T) {
	text := "token: ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	if !ScanMessage(text, DefaultPatterns) {
		t.Error("expected GitHub token to be detected")
	}
}

func TestRedactPreservesNonSecrets(t *testing.T) {
	text := "Here is some ordinary text without any secrets."
	redacted := RedactSecrets(text, DefaultPatterns)
	if text != redacted {
		t.Errorf("non-secret text was modified: %q", redacted)
	}
}

func TestRedactReplacesSecret(t *testing.T) {
	text := "ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	redacted := RedactSecrets(text, DefaultPatterns)
	if strings.Contains(redacted, "ghp_") {
		t.Error("GitHub token not redacted")
	}
	if !strings.Contains(redacted, "[SECRET:") {
		t.Error("expected [SECRET:...] placeholder")
	}
}
