# Phase 14 — Rich Input: Suggestions, Autocomplete, Pickers

Goal: upgrade the TUI prompt from a plain single-line `textinput` into a
completion-aware editor with inline ghost suggestions, a trigger-driven dropdown
menu, and `huh`-based select / multi-select pickers for commands where free text
is the wrong affordance.

Design only. No code shipped in this doc.

## Why

The Charm stack we already build on (Bubble Tea + Bubbles + Lip Gloss + Huh) is
the same stack `charmbracelet/crush` uses for its coding-agent UI. Crush's
prompt is a `bubbles/list` rendered in a floating `lipgloss` box above the
editor, fed by trigger tokens (`/`, `@`). We rebuild that pattern in our own
code — Crush is **FSL-licensed** (source-available, not MIT), so we study the
design and write our own; we copy no Crush source. The libraries themselves are
MIT and already in `go.mod`.

## Current state

- `internal/ui/tui/model.go`: single-line `bubbles/textinput`, suggestions
  **disabled**. Slash commands parsed by hand in `handleSlashCommand` (~L287).
- `/model <name>` etc. take raw free text; `/model` alone prints usage instead
  of offering a picker.

## Dependencies — all already present

| Need                     | Lib                  | Status in go.mod |
| ------------------------ | -------------------- | ---------------- |
| Inline ghost suggestion  | `bubbles/textinput`  | present, OFF     |
| Fuzzy ranking            | `sahilm/fuzzy`       | indirect dep     |
| Scrollable dropdown list | `bubbles/list`       | available        |
| Select / MultiSelect     | `huh`                | direct dep       |
| Floating box render      | `lipgloss`           | direct dep       |

No new modules required for Tiers 1–3. Catwalk (provider/model catalog) is an
optional later data source for the model provider.

## Confirmed Bubbles API (verified against bubbles@v1.0.0)

`textinput.Model` supports suggestions natively:

- field `ShowSuggestions bool`
- `SetSuggestions([]string)`
- `CurrentSuggestion() string`
- keybindings: `AcceptSuggestion`=Tab, `NextSuggestion`=down/ctrl+n,
  `PrevSuggestion`=up/ctrl+p

Limit: prefix match, one ghost completion + cycle, **no visible menu**. Good
enough for Tier 1; Tier 2 adds the menu.

## Architecture

A `CompletionProvider` registry drives both the inline suggestions and the
dropdown. Each provider inspects the line + cursor, decides whether it owns the
current token, and returns ranked items.

```go
// internal/ui/tui/completions/provider.go
package completions

type Item struct {
    Title  string // shown in the menu
    Desc   string // right-aligned hint / description
    Insert string // text written into the input on accept
}

type Provider interface {
    // Trigger reports whether this provider is active for the given line and
    // cursor, the query substring to match on, and the byte offset where the
    // replaced token starts (so accept can splice cleanly).
    Trigger(line string, cursor int) (active bool, query string, start int)

    // Items returns fuzzy-ranked candidates for the query.
    Items(query string) []Item
}
```

Providers to implement (all four triggers requested):

| Trigger              | Provider        | Source                                              |
| -------------------- | --------------- | --------------------------------------------------- |
| `/` at col 0         | `SlashProvider` | static command table (name + description)           |
| `@`                  | `FileProvider`  | sandbox-scoped walk via `internal/tools/fs`, fuzzy  |
| model name arg       | `ModelProvider` | Ollama client list; later Catwalk catalog           |
| `#`                  | `SkillProvider` | `engine.LoadedSkills()` (+ optional code symbols)   |

`ModelProvider` activates contextually: when the line is `/model ` it owns the
trailing token. `SkillProvider` mirrors `FileProvider` but keys on `#`.

### Dropdown component

```go
// internal/ui/tui/completions/model.go
type Model struct {
    list      list.Model      // bubbles/list, fuzzy filter on
    providers []Provider
    active    Provider
    start     int             // splice offset from Trigger
    visible   bool
}
```

- `Update(msg, line, cursor)`: re-run `Trigger` across providers each keystroke;
  first active provider wins; refill `list` with its `Items`; set `visible`.
- Rendered in a bordered `lipgloss` box positioned directly above the input row
  (the parent `View()` stacks: viewport / completions box / input).
- Keys when visible: ↑↓ navigate, Enter/Tab accept, Esc dismiss. Accept returns
  `(insert string, start int)` so the parent splices into `textInput.Value()`.

### Wiring into the existing model

In `internal/ui/tui/model.go`:

1. `New()`: `ti.ShowSuggestions = true`; construct `completions.Model` with the
   four providers; store on `Model`.
2. `Update` `tea.KeyMsg`: when the completions menu is `visible`, route ↑↓/Enter/
   Tab/Esc to it first; otherwise fall through to existing handling. After every
   key, call `completions.Update(line, cursor)` to refresh state. Also feed the
   active provider's items into `m.textInput.SetSuggestions(...)` so inline ghost
   text and the menu stay consistent.
3. `View`/`renderInput` (~L694): insert the completions box above the input when
   visible.

### Pickers (Tier 3, `huh`)

For commands where a menu beats free text, run a `huh` form as an embedded
Bubble Tea model (huh composes with our `Update`):

- `/model` with no arg → `huh.NewSelect[string]()` of model names.
- `/tools` → `huh.NewMultiSelect[string]()` to enable/disable a tool set.
- approval flow (`approve.go`) → `huh` confirm/select for tool-call gating.

State machine: an `inputMode` enum (`modeChat | modePicker`) on `Model`; while
`modePicker`, the form owns `Update`/`View`; on completion it emits a `tea.Msg`
carrying the selection back into the chat flow.

## Build sequence

1. **Tier 1 — inline ghost suggest.** `ShowSuggestions=true` + per-keystroke
   `SetSuggestions` from the slash/model/tool/skill name sets. One file, low risk.
2. **Tier 2 — dropdown.** New `completions` package: `Item`, `Provider`,
   `SlashProvider`, `FileProvider`, `ModelProvider`, `SkillProvider`, `Model`.
   Wire into `model.go` `Update`/`View`. Unit-test each provider's `Trigger`
   offset math and fuzzy ranking.
3. **Tier 3 — pickers.** `inputMode` state machine + `huh` forms for `/model`,
   `/tools`, and approvals.

## Testing

- Provider tests: table-driven `Trigger(line, cursor)` → `(active, query, start)`
  including edge cases (cursor mid-token, escaped `/`, empty query).
- Accept-splice test: given input + selection, assert resulting `textInput`
  value and cursor.
- `teatest` (bubbletea) golden test for the rendered dropdown box.

## Open questions

- Symbol completion for `#` (beyond skills) needs an LSP/index source — defer to
  the Crush-LSP integration track; ship `#`=skills first.
- File walk perf on large trees — cap depth / debounce, reuse fs sandbox limits.
- Catwalk adoption for `ModelProvider` — separate decision; Ollama list works now.

## Cross-references

- Charm/Crush evaluation: this repo's runtime/providers tracks.
- `internal/tools/fs` for sandbox-scoped file enumeration (FileProvider source).
- `internal/ui/tui/approve.go` for the approval picker (Tier 3).
