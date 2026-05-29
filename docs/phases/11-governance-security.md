# Phase 11 — Governance & Security

Tags: `#security` `#auth` `#authz` `#pii` `#guardrails` `#compliance` `#audit` `#p2` `#status/planned`

Prerequisites: Phase 05 (audit log), Phase 01 (policy engine).

---

## Goal

Harden the runtime with: authentication/authorization for API access, PII detection and
scrubbing, model/prompt guardrails, compliance-grade audit trail, and secret scanning to
prevent accidental leakage of API keys through the agent.

---

## Files to create (in order)

### Step 1 — Auth tokens
**`internal/governance/auth.go`**

For API/SDK access mode (when running as a server rather than TUI):

```go
package governance

type Identity struct {
    UserID  string
    OrgID   string
    Roles   []string
    Token   string
}

type AuthProvider interface {
    Verify(ctx context.Context, token string) (Identity, error)
}

// StaticTokenAuth: reads tokens from config, suitable for local/single-user.
type StaticTokenAuth struct {
    tokens map[string]Identity
}

func NewStaticTokenAuth(tokens map[string]Identity) *StaticTokenAuth
func (a *StaticTokenAuth) Verify(ctx context.Context, token string) (Identity, error)
```

Tags: `#governance/auth`

---

### Step 2 — Authorization
**`internal/governance/authz.go`**

```go
type Permission string

const (
    PermRead        Permission = "read"
    PermWrite       Permission = "write"
    PermExecute     Permission = "execute"
    PermApprove     Permission = "approve"
    PermAdmin       Permission = "admin"
)

type Authorizer interface {
    Can(identity Identity, perm Permission, resource string) bool
}

// RBACAuthorizer: role-based access control.
// Roles: "viewer" (read only), "operator" (read+write+execute), "admin" (all).
type RBACAuthorizer struct {
    rolePermissions map[string][]Permission
}

func NewRBACAuthorizer() *RBACAuthorizer
func (a *RBACAuthorizer) Can(identity Identity, perm Permission, resource string) bool
```

Wire into `policy.DefaultEngine.Enforce()`: check authorizer before approval callback.

Tags: `#governance/authz`

---

### Step 3 — PII detector
**`internal/governance/pii.go`**

```go
type PIIKind string

const (
    PIIEmail       PIIKind = "email"
    PIIPhone       PIIKind = "phone"
    PIISSN         PIIKind = "ssn"
    PIICreditCard  PIIKind = "credit_card"
    PIIIPAddress   PIIKind = "ip_address"
)

type PIIMatch struct {
    Kind  PIIKind
    Start int
    End   int
    Value string
}

// Detect returns all PII matches found in text.
func Detect(text string) []PIIMatch

// Scrub replaces all PII matches with [REDACTED:<kind>].
func Scrub(text string) string
```

Uses compiled `regexp.Regexp` patterns — no external dependency.

Apply scrubbing:
- **Inbound**: scrub user input before sending to LLM (configurable, default off)
- **Outbound**: scrub tool results that contain file content (configurable)
- **Storage**: scrub before persisting to messages table (configurable)

Tags: `#governance/pii`

---

### Step 4 — Secret scanner
**`internal/governance/secrets.go`**

Prevent the agent from accidentally echoing API keys, tokens, or credentials:

```go
type SecretPattern struct {
    Name    string
    Pattern *regexp.Regexp
}

// DefaultPatterns covers: AWS keys, GitHub tokens, OpenAI keys, Anthropic keys,
// generic "key = ...", bearer tokens.
var DefaultPatterns []SecretPattern

// ScanMessage returns true if text contains a likely secret.
func ScanMessage(text string, patterns []SecretPattern) bool

// RedactSecrets replaces matched secrets with [SECRET:<name>].
func RedactSecrets(text string, patterns []SecretPattern) string
```

Wire into `tools.Host.Execute()`: scan tool results before returning to agent.
If secret detected: log audit event, optionally block (configurable).

Tags: `#governance/secrets`

---

### Step 5 — Prompt guardrails
**`internal/governance/guardrails.go`**

```go
type GuardrailResult struct {
    Blocked bool
    Reason  string
}

type Guardrail interface {
    CheckInput(ctx context.Context, input string) GuardrailResult
    CheckOutput(ctx context.Context, output string) GuardrailResult
}

// KeywordGuardrail: blocks inputs/outputs matching configurable keyword lists.
type KeywordGuardrail struct {
    BlockedInputPatterns  []*regexp.Regexp
    BlockedOutputPatterns []*regexp.Regexp
}

// LLMGuardrail: uses a fast/cheap LLM call to classify harmfulness.
// Falls back to allow on LLM error to avoid blocking legitimate use.
type LLMGuardrail struct {
    provider providers.Provider
    model    string
}
```

Wire into `Engine.Submit()`:
1. Check `guardrails.CheckInput(input)` → if blocked, return error without running agent
2. Check `guardrails.CheckOutput(result)` → if blocked, return sanitized refusal message

Tags: `#governance/guardrails`

---

### Step 6 — Compliance config
**`internal/governance/compliance.go`**

```go
type ComplianceConfig struct {
    PIIScrubInput    bool
    PIIScrubOutput   bool
    PIIScrubStorage  bool
    SecretScanTools  bool
    SecretBlockOnHit bool
    AuditAllMessages bool
    RetentionDays    int    // 0 = keep forever
}

// Purge deletes audit/span/message records older than RetentionDays.
func Purge(ctx context.Context, db *sql.DB, cfg ComplianceConfig) (int64, error)
```

Tags: `#governance/compliance`

---

### Step 7 — Wire into bootstrap
**`internal/app/bootstrap.go`** — add governance wiring:

```go
gov := governance.New(cfg.Governance)
gov.RegisterGuardrails(engine)
gov.RegisterPIIScrubber(engine.ToolHost)
gov.RegisterSecretScanner(engine.ToolHost)
```

Tags: `#governance/bootstrap`

---

### Step 8 — `/compliance` TUI command
**`internal/ui/tui/model.go`** — add `/compliance purge` and `/compliance report`:

```
/compliance report   → show PII scrubs, secrets blocked, audit summary
/compliance purge    → run Purge() with configured retention
```

Tags: `#ui/tui` `#governance/cli`

---

## Tests

**`internal/governance/pii_test.go`**
- `TestDetectEmail`
- `TestDetectPhone`
- `TestScrubReplacesAll`
- `TestDetectNegative` — plain text returns no matches

**`internal/governance/secrets_test.go`**
- `TestScanAWSKey`
- `TestScanGitHubToken`
- `TestRedactPreservesNonSecrets`

**`internal/governance/guardrails_test.go`**
- `TestKeywordGuardrailBlocks`
- `TestKeywordGuardrailAllows`

Tags: `#tests`

---

## Verification

```
go test ./internal/governance/...
```

- Send input with SSN "123-45-6789" (scrub enabled) → audit log shows PIIScrubbed event
- Tool returns file with AWS key → SecretScanner fires → result redacted in LLM context
- `BlockedInputPattern` matching → Submit returns error before LLM call (verify with span: no SpanLLMCall)
- `/compliance report` shows running totals
