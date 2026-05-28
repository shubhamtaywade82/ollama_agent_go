# Ollama Agent Go: Full Replacement Plan

This document outlines the roadmap to reach 100% feature parity with the Ruby `ollama_agent` gem and its advanced "Kernel" runtime, ultimately replacing it with a high-performance, reactive Go binary.

## Phase 1: Core Agent Loop & Tool Calling (Parity with stable gem)
**Goal:** Implement the fundamental "think-act" loop and sandboxed tool execution.

1. **Tool Registry (`pkg/tools`)**
   - Create a `Tool` interface with `Name()`, `Description()`, `Parameters()`, and `Execute`.
   - Implement core tools: `read_file`, `write_file`, `ls`, `grep_search`, `run_shell`.
   - Implement the `edit_file` (Unified Diff) tool using a native Go diff library.

2. **Regex & JSON Tool Dispatcher**
   - Support both standard JSON tool-calling and "Synthetic" parsing for models like Gemma/Llama 3 that output Markdown code blocks.
   - Implement a robust dispatcher with multi-turn capability (agent can call tools repeatedly).

3. **Context & Token Management**
   - Port the sliding window context trimmer.
   - Implement token counting using a Go-native tiktoken/GPT tokenizer library.
   - Support "Skills" ingestion (Markdown files that inject system instructions).

## Phase 2: Advanced Context & Command System
**Goal:** Port the CLI-level interactive features.

1. **Slash Command Engine**
   - Parser for `/help`, `/model`, `/session`, `/clear`, `/config`, `/provider`.
   - Implement the `/models` command with capability heuristics (Tools/Vision/Reasoning).

2. **Repository Indexing (Topological Search)**
   - Implement a code-aware indexer (analogous to the Ruby Prism index).
   - Support semantic/symbol-based search for Go, Ruby, and Python.

3. **Multi-Provider Support**
   - Implement adapters for OpenAI, Anthropic, Groq, and OpenRouter.
   - Build the "Cloud Router" for automatic fallback and cost tracking.

## Phase 3: The "Kernel" Runtime (Advanced Features)
**Goal:** Port the production-grade reliability layer.

1. **Saga Coordinator & WAL**
   - Implement a Write-Ahead Log (WAL) to track every file mutation.
   - Implement the Saga State Machine (Reserved → Locked → Applied → Verified → Committed).
   - Support compensation (automatic rollback) on tool failure.

2. **Database & Persistence**
   - Integrate SQLite for session storage, cost ledgers, and mutation history.
   - Implement a schema migrator to manage database versions.

3. **Ownership & Security Gates**
   - Port the `owners.yml` logic to enforce restricted access to sensitive directories.
   - Implement the Approval Gate for high-risk operations.

## Phase 4: Special Modes & Distribution
**Goal:** Reach total parity on specialized CLI workflows.

1. **Orchestrator Mode**
   - Support delegation to external agents (Claude CLI, Gemini CLI) with interactive confirmation.

2. **Self-Improvement Sandbox**
   - Port the `self_review` and `improve` commands.
   - Implement isolated validation (Docker-based) for testing agent-generated code.

3. **Plugin Architecture**
   - Implement a dynamic loading system for custom Go-based tools and commands.

4. **Distribution**
   - Provide pre-compiled binaries for Linux, macOS, and Windows.
   - Performance: Target < 30ms startup and < 25MB memory footprint.

## Success Criteria for "Total Replacement"
- [ ] Support for all OLLAMA_AGENT_* environment variables.
- [ ] 1:1 parity with Ruby gem's TUI and REPL commands.
- [ ] Support for Kernel Saga-based mutation workflows.
- [ ] Pass full suite of integration tests (porting Ruby `spec/` to Go `*_test.go`).

## Phase 5: Aesthetic & Rich UI (via Charm.land)
**Goal:** Use high-fidelity terminal components to create a "next-gen" terminal experience.

1. **Reactive Log Dashboard (`charmbracelet/log`)**
   - Replace standard fmt.Print with a structured, colorful log.
   - Use log levels and prefixes to distinguish between LLM reasoning, Tool execution, and Kernel state changes.

2. **Interactive Selection & Filtering (`charmbracelet/bubbles/list`)**
   - Use rich lists for `/model list` and `/session list` with fuzzy searching.
   - Implement the Approval Gate as a high-fidelity "Confirm Action" card.

3. **Multi-Column Layouts (`charmbracelet/lipgloss`)**
   - Create a dashboard view with a sidebar showing:
     - Real-time Token Cost (LED-style digits).
     - Active Tool status (Spinning indicators).
     - Kernel Saga status (Progress bars).

4. **Markdown Mastery (`charmbracelet/glamour`)**
   - Render LLM responses with full syntax highlighting and theme support directly in the TUI.
