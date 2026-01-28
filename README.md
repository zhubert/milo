# Milo

Milo is an exploratory project for understanding how to build a coding agent CLI. It implements the core patterns that power AI-assisted development tools: an agentic loop, tool execution, permission management, and a terminal interface.

## What is a Coding Agent?

A coding agent is a loop that alternates between asking an LLM "what should I do next?" and executing the actions it requests—reading files, writing code, running commands. The entire architecture exists to support this loop while providing a good user experience around it.

```
┌─────────────────────────────────────────────────────┐
│                    Terminal UI (TUI)                │
│         User input, streaming display, dialogs      │
├─────────────────────────────────────────────────────┤
│                   Session Manager                   │
│        Message history, persistence, branching      │
├──────────────┬──────────────────┬───────────────────┤
│  Agent Loop  │   Tool Registry  │  Permission System│
│  (the core)  │  built-in tools  │  allow/deny/ask   │
├──────────────┴──────────────────┴───────────────────┤
│              LLM Provider Abstraction               │
│         Anthropic, OpenAI, Google, etc.             │
├─────────────────────────────────────────────────────┤
│              Supporting Infrastructure              │
│  Config, Storage, LSP, File Watcher, Plugins, Bus   │
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
│   ├── app/          # Application orchestration
│   ├── logging/      # Structured logging via log/slog
│   ├── permission/   # Permission system for tool execution
│   ├── tool/         # Tool definitions and registry
│   └── ui/           # Terminal UI (Bubble Tea)
├── main.go           # Entry point
└── go.mod
```

## Core Concepts

### Tools

Tools are how the agent interacts with the real world. Each tool has:

- A unique ID and description (the LLM reads this to decide when to use it)
- An input schema for validation
- An execute function that does the work and returns a result

Common tool categories:

| Category   | Tools              | Purpose                        |
| ---------- | ------------------ | ------------------------------ |
| File I/O   | read, write, edit  | Read and modify files          |
| Search     | glob, grep         | Find files and search content  |
| Execution  | bash               | Run shell commands             |

### Permissions

A permission system controls what the agent can do:

- **Allow** — auto-approve the action
- **Deny** — auto-reject the action
- **Ask** — prompt the user for approval

This provides safety guardrails so the agent can't run arbitrary commands without oversight.

### Session Management

Conversations are persisted as sessions with full history. This enables:

- Resuming previous conversations
- Context window management (compaction when history is too long)
- Undo/redo of conversation state

## Tech Stack

- **Go** — the implementation language
- **Bubble Tea** — terminal UI framework
- **Lip Gloss** — terminal styling
- **Anthropic SDK** — LLM provider integration
- **Cobra** — CLI framework

## Running

```bash
go build -o milo .
./milo
```

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
2. **Tool execution** — validating inputs, managing permissions, handling errors
3. **Context management** — fitting conversation history within token limits
4. **Terminal UX** — building an interactive experience with markdown rendering, syntax highlighting, and responsive input
5. **State persistence** — saving and resuming sessions reliably

Milo is a sandbox for exploring these patterns and understanding what makes a coding agent work.

## License

MIT
