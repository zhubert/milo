# Milo

Milo is an exploratory project for understanding how to build a coding agent CLI. It implements the core patterns that power AI-assisted development tools: an agentic loop, tool execution, permission management, and a terminal interface.

## What is a Coding Agent?

A coding agent is a loop that alternates between asking an LLM "what should I do next?" and executing the actions it requests—reading files, writing code, running commands. The entire architecture exists to support this loop while providing a good user experience around it.

```
┌─────────────────────────────────────────────────────┐
│                      Runner                         │
│       Readline-based input, streaming display       │
├─────────────────────────────────────────────────────┤
│                   Session Manager                   │
│          Message history and persistence            │
├──────────────┬──────────────────┬───────────────────┤
│  Agent Loop  │   Tool Registry  │  Permission System│
│  (the core)  │  built-in tools  │  allow/deny/ask   │
├──────────────┴──────────────────┴───────────────────┤
│              LLM Provider Abstraction               │
│                     Anthropic                       │
├─────────────────────────────────────────────────────┤
│              Supporting Infrastructure              │
│  LSP, Context Management, Todo Tracking, Logging    │
└─────────────────────────────────────────────────────┘
```

## The Agentic Loop

The most important piece of any coding agent is its central loop:

1. **Gather context** — the user's message, conversation history, system prompt, and available tools
2. **Call the LLM** via streaming so responses appear in real time
3. **Process the stream** — as text and tool calls arrive, render them to the user
4. **Execute tool calls** — when the LLM requests a tool (e.g., "read file X"), execute it and feed the result back
5. **Decide what to do next:**
   - If the LLM finished with text (no pending tool calls) → exit the loop
   - If tools were called → continue the loop with results
   - If context is too large → compact (summarize) and continue

The LLM acts as the "brain" deciding what to do, and the tools are the "hands" that interact with the filesystem and environment.

## Project Structure

```
milo/
├── cmd/              # CLI entry point (Cobra commands)
├── internal/
│   ├── agent/        # The agentic loop implementation
│   ├── context/      # Context window management and summarization
│   ├── logging/      # Structured logging via log/slog
│   ├── loopdetector/ # Doom loop detection (stuck agent patterns)
│   ├── lsp/          # Language Server Protocol integration
│   ├── permission/   # Permission system for tool execution
│   ├── runner/       # Readline-based CLI runner
│   ├── session/      # Session persistence and history
│   ├── todo/         # Task list management for the agent
│   ├── token/        # Token counting and estimation
│   ├── tool/         # Tool definitions and registry
│   └── version/      # Build version information
├── main.go           # Entry point
├── Makefile          # Build, test, and development tasks
└── go.mod
```

## Core Concepts

### Tools

Tools are how the agent interacts with the real world. Each tool has:

- A unique ID and description (the LLM reads this to decide when to use it)
- An input schema for validation
- An execute function that does the work and returns a result

Common tool categories:

| Category   | Tools                              | Purpose                              |
| ---------- | ---------------------------------- | ------------------------------------ |
| File I/O   | read, multiread, write, edit, move | Read, modify, and move files         |
| File Info  | diff, undo                         | View changes and revert edits        |
| Navigation | glob, grep, listdir, tree          | Find files and explore directories   |
| Execution  | bash, git                          | Run shell commands and git operations|
| Web        | webfetch, websearch                | Fetch URLs and search the web        |
| LSP        | lsp                                | Language server queries (hover, etc.)|
| Planning   | todo                               | Task list management                 |

### Permissions

A permission system controls what the agent can do:

- **Allow** — auto-approve the action
- **Deny** — auto-reject the action
- **Ask** — prompt the user for approval

This provides safety guardrails so the agent can't run arbitrary commands without oversight.

### Session Management

Conversations are persisted as sessions with full history. This enables:

- Resuming previous conversations
- Browsing session history with `milo sessions`

### Context Window Management

As conversations grow, the agent automatically manages the context window:

- Token counting tracks usage against model limits
- When approaching limits, older messages are summarized using Claude Haiku
- Recent messages are preserved intact for continuity

## Tech Stack

- **Go** — the implementation language
- **Anthropic SDK** — LLM provider integration
- **Cobra** — CLI framework
- **Readline** — terminal input handling
- **Glamour** — markdown rendering

## Installation

### Homebrew

```bash
brew install zhubert/tap/milo
```

### From Source

```bash
go build -o milo .
./milo
```

## Usage

```bash
# Start a new session
milo

# Start a fresh session (ignore any existing)
milo --new

# Resume a specific session by ID
milo --resume <session-id>

# Resume the most recent session
milo --resume last

# Use a specific Claude model
milo -m claude-opus-4-5-20251101
milo --model claude-sonnet-4-20250514

# List all saved sessions
milo sessions
```

### In-Session Commands

| Command                  | Description                          |
| ------------------------ | ------------------------------------ |
| `/model`, `/m`           | Interactive model selection          |
| `/model <id>`            | Switch to a specific model           |
| `/permissions`, `/p`     | Manage permission rules              |
| `/help`, `/h`            | Show available commands              |
| `exit`, `quit`           | Close the application                |

## Development

This project follows idiomatic Go conventions. See [CLAUDE.md](CLAUDE.md) for coding guidelines.

```bash
# Run tests
go test ./...

# Format code
gofmt -w .

# Tidy dependencies
go mod tidy
```

## What This Project Explores

Building a coding agent requires solving several interesting problems:

1. **Streaming LLM responses** — rendering text as it arrives while handling tool calls mid-stream
2. **Tool execution** — validating inputs, managing permissions, handling errors, parallel execution
3. **Context management** — fitting conversation history within token limits via automatic summarization
4. **Language server integration** — leveraging LSP for hover info, diagnostics, and code intelligence
5. **Terminal UX** — building an interactive experience with markdown rendering and responsive input
6. **State persistence** — saving and resuming sessions reliably

Milo is a sandbox for exploring these patterns and understanding what makes a coding agent work.

## License

MIT
