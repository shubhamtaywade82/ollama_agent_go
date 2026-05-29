package governance

// ComplianceConfig controls runtime data-handling policies.
type ComplianceConfig struct {
	PIIScrubInput    bool // scrub PII from user input before sending to LLM
	PIIScrubOutput   bool // scrub PII from LLM output before returning to user
	PIIScrubStorage  bool // scrub PII before persisting messages
	SecretScanTools  bool // scan tool results for secrets
	SecretBlockOnHit bool // block tool result if secret detected (else just redact)
	AuditAllMessages bool // write every message to the audit log
	RetentionDays    int  // 0 = keep forever; >0 = purge older records
}

// DefaultCompliance is the out-of-the-box compliance configuration (permissive).
var DefaultCompliance = ComplianceConfig{
	PIIScrubInput:    false,
	PIIScrubOutput:   false,
	PIIScrubStorage:  false,
	SecretScanTools:  true,
	SecretBlockOnHit: false,
	AuditAllMessages: false,
	RetentionDays:    0,
}
