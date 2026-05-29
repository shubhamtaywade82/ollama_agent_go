package governance

// Governor bundles all governance components and exposes helpers for wiring
// them into the Engine and ToolHost.
type Governor struct {
	Auth       AuthProvider
	Authz      Authorizer
	Guardrail  Guardrail
	Compliance ComplianceConfig
	Secrets    []SecretPattern
}

// New creates a Governor with default settings (auth disabled, secret scanning enabled).
func New(cfg ComplianceConfig) *Governor {
	return &Governor{
		Auth:       NoAuth{},
		Authz:      AllowAll{},
		Guardrail:  NoopGuardrail{},
		Compliance: cfg,
		Secrets:    DefaultPatterns,
	}
}

// ScrubInput applies PII scrubbing to user input if configured.
func (g *Governor) ScrubInput(text string) string {
	if g.Compliance.PIIScrubInput {
		return Scrub(text)
	}
	return text
}

// ScrubOutput applies PII scrubbing to LLM output if configured.
func (g *Governor) ScrubOutput(text string) string {
	if g.Compliance.PIIScrubOutput {
		return Scrub(text)
	}
	return text
}

// ScrubStorage applies PII scrubbing to message content before storage if configured.
func (g *Governor) ScrubStorage(text string) string {
	if g.Compliance.PIIScrubStorage {
		return Scrub(text)
	}
	return text
}

// ScanToolResult checks a tool result for secrets. Returns the (possibly redacted)
// result and whether a secret was found.
func (g *Governor) ScanToolResult(result string) (out string, found bool) {
	if !g.Compliance.SecretScanTools {
		return result, false
	}
	if ScanMessage(result, g.Secrets) {
		return RedactSecrets(result, g.Secrets), true
	}
	return result, false
}
