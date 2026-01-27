# How a Coding Agent CLI Works: Architecture Deep Dive

A coding agent is a loop that alternates between asking an LLM "what should I do next?" and executing the actions it requests — reading files, writing code, running commands. The entire architecture exists to support this loop while providing a good user experience around it.

## Core Components

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

---

## 1. The Agentic Loop

The most important piece of any coding agent is its central loop. This is a `while true` loop that:

1. **Gathers context** — the user's message, conversation history, system prompt, and available tools.
2. **Calls the LLM** via streaming so responses appear in real time.
3. **Processes the stream** — as text and tool calls arrive, they're rendered to the user.
4. **Executes tool calls** — when the LLM requests a tool (e.g., "read file X"), the system executes it and feeds the result back as a new message.
5. **Decides what to do next:**
   - If the LLM finished with text (no pending tool calls) → **exit the loop**, show the result.
   - If tools were called → **continue the loop**, let the LLM see the results and decide next steps.
   - If the conversation context is too large → **compact** (summarize the conversation) and continue.
   - If a "doom loop" is detected (same tool called repeatedly with identical input) → interrupt and prompt the user.

This is the fundamental pattern behind every coding agent. The LLM acts as the "brain" deciding what to do, and the tools are the "hands" that interact with the filesystem and environment.

### Pseudocode

```
function agentLoop(userMessage, session):
    addToHistory(session, userMessage)

    while true:
        tools = resolveAvailableTools(session.agent)
        systemPrompt = buildSystemPrompt(session)
        messages = session.history

        stream = llm.stream(systemPrompt, messages, tools)

        for event in stream:
            if event.type == "text":
                renderText(event.content)

            if event.type == "tool_call":
                checkPermission(event.tool, event.args)
                result = executeTool(event.tool, event.args)
                addToolResult(session, event.callId, result)

            if event.type == "finish":
                if event.reason == "end_of_text":
                    return  // done
                if event.reason == "tool_calls_complete":
                    continue  // loop again with tool results
                if contextTooLarge(session):
                    compact(session)
                    continue
```

---

## 2. Tool System

Tools are how the agent interacts with the real world. Each tool is defined with:

- A **unique ID** and **description** (the LLM reads this to decide when to use it)
- An **input schema** (JSON Schema) for validation
- An **execute function** that does the actual work and returns a result

### Standard Tools

| Category          | Tools                                  | Purpose                                                   |
| ----------------- | -------------------------------------- | --------------------------------------------------------- |
| File I/O          | `read`, `write`, `edit`, `apply_patch` | Read, create, and modify files                            |
| Search            | `glob`, `grep`                         | Find files and search content                             |
| Execution         | `bash`                                 | Run shell commands                                        |
| Web               | `webfetch`, `websearch`                | Access the internet                                       |
| User interaction  | `question`                             | Ask the user for clarification                            |
| Task management   | `task`, `todo`                         | Track multi-step work                                     |
| Code intelligence | `lsp`                                  | Hover info, go-to-definition via Language Server Protocol |
| Orchestration     | `batch`, `skill`                       | Parallel tool execution, custom skills                    |

### Tool Execution Flow

```
LLM emits tool_call
  → validate input against schema
  → check permissions (allow / deny / ask user)
  → execute the tool
  → return structured result {title, output, metadata, attachments}
  → feed result back to LLM as a tool_result message
```

### Tool Registration

Tools are managed through a **registry** that aggregates:

- Built-in tools (the standard set above)
- Plugin-provided tools (loaded from a plugin system)
- User-defined custom tools (loaded from config directories)
- MCP tools (dynamically discovered from Model Context Protocol servers)

The registry resolves which tools are available based on the current agent mode and permission rules.

---

## 3. LLM Provider Abstraction

A provider abstraction layer gives the agent a single interface to call any supported LLM. This layer handles:

- **Authentication** — environment variables, config files, OAuth flows (e.g., GitHub Copilot tokens)
- **API differences** — each provider has different endpoints, headers, and request formats
- **Model capabilities** — which models support tool calling, reasoning/thinking, image input, caching, etc.
- **Message format normalization** — tool call IDs, reasoning content fields, and cache control headers differ across providers
- **Streaming protocols** — SSE formats and chunk structures vary

### Provider-Specific Transformations

Because providers differ in subtle ways, a transformation layer handles:

- **Message normalization** — e.g., Anthropic requires non-empty content arrays; some providers have different tool call ID formats
- **Cache control** — Anthropic uses `cacheControl: {type: "ephemeral"}`, Bedrock uses `cachePoint`, OpenAI uses `cache_control`
- **Reasoning content** — some providers expose chain-of-thought as `reasoning_content`, others as `reasoning_details`
- **Temperature and sampling** — recommended values differ per model

---

## 4. System Prompt Construction

The system prompt turns a general-purpose LLM into a coding assistant. It is assembled from multiple pieces:

1. **Base prompt** — provider-specific instructions optimized for each model family (Claude, GPT, Gemini, etc.)
2. **Environment context** — injected dynamically:
   ```
   Working directory: /path/to/project
   Is directory a git repo: yes
   Platform: darwin
   Today's date: 2025-01-15
   Model: claude-sonnet-4-20250514
   ```
3. **Agent-specific instructions** — different agents (build, plan, explore) get different behavioral instructions and tool access descriptions
4. **User-provided system instructions** — custom instructions from configuration
5. **Plugin transformations** — plugins can modify the system prompt via hooks

The system prompt is often structured in two parts for **prompt caching**: a static header that rarely changes (and can be cached by the provider) and a dynamic tail with environment-specific content.

---

## 5. Agent Modes

Rather than one monolithic agent, the system defines multiple **agent modes** with different capabilities and constraints:

| Agent       | Purpose                                           | Tool Access                         |
| ----------- | ------------------------------------------------- | ----------------------------------- |
| **Build**   | Full development — read, write, execute           | All tools                           |
| **Plan**    | Analysis and planning — explore but don't modify  | Read-only tools, no edit/write/bash |
| **Explore** | Quick research — lightweight codebase exploration | grep, glob, read only               |
| **General** | Delegated complex subtasks                        | Most tools, no task management      |

Each agent has configurable properties:

- **Temperature and sampling parameters**
- **Maximum steps** (loop iterations before forced stop)
- **Permission rules** (which tools require user approval)
- **System prompt overrides**

---

## 6. Permission System

A permission system controls what the agent can do, providing safety guardrails:

### Rule Structure

Each rule specifies:

- **Permission** — which tool or action (e.g., `bash`, `edit`, `read`, `external_directory`)
- **Pattern** — what resource the rule applies to (glob patterns for file paths, command prefixes for bash)
- **Action** — `allow` (auto-approve), `deny` (auto-reject), or `ask` (prompt the user)

### Evaluation

- Rules are evaluated in order; the last matching rule wins
- If no rule matches, the default is `ask`
- "Always allow" choices are persisted per-project so the user isn't repeatedly prompted
- Different agent modes have different default permission sets (e.g., the plan agent denies all edit operations)

### Doom Loop Detection

If the agent calls the same tool with identical input three times in a row, the system triggers an automatic permission check. This prevents infinite loops where the agent repeatedly tries a failing action.

### User Interaction

When a permission check requires user input, the loop pauses and presents three options:

- **Allow once** — approve this specific invocation
- **Allow always** — persist the rule for this project
- **Reject** — deny the action (optionally with a guidance message the agent will see)

---

## 7. Session and Message Management

Conversations are persisted as **sessions** with full history:

### Message Structure

Each message has:

- **Role** — `user` or `assistant`
- **Parts** — a list of typed content blocks:
  - `text` — regular text content
  - `reasoning` — model chain-of-thought / extended thinking
  - `tool_invocation` — a tool call with its status, input, and output
  - `file` — attached files or images
  - `patch` — file diffs generated during the session
  - `compaction` — a summary that replaced earlier messages

### Persistence

- Sessions are stored as JSON files in a platform-appropriate data directory (e.g., `~/.local/share/` on Linux)
- File-level locking handles concurrent access
- Version-based migrations handle schema evolution

### Context Management

- **Compaction** — when conversation history exceeds the model's context window, older messages are summarized into a compact representation
- **Branching** — users can fork a session from any message, creating conversation branches
- **Undo/redo** — conversation state can be reverted to earlier points
- **Snapshots** — the file system state is snapshotted at step boundaries, enabling rollback of file changes

### Message Format Conversion

An internal message format is converted to provider-specific formats when sending to the LLM. This handles differences in how providers represent tool calls, reasoning content, and multi-part messages.

---

## 8. Terminal User Interface

The TUI provides the interactive experience:

### Rendering

- **Streaming display** — LLM responses render character-by-character as they arrive
- **Markdown rendering** — full markdown with syntax-highlighted code blocks in the terminal
- **Tool visualization** — each tool type has a custom renderer (bash shows command + output, edit shows diffs, etc.)

### Input

- **Multi-line text input** with prompt history and undo/redo
- **Slash commands** — `/new`, `/models`, `/compact`, `/sessions`, etc.
- **Autocomplete** for commands and file references
- **Configurable keybindings** — vim-style navigation, leader key support

### UI Components

- **Command palette** — searchable list of all available commands
- **Modal dialogs** — model selection, session switching, permission prompts
- **Scrollable message history** with page/half-page/line navigation
- **Sidebar** — session list and metadata
- **Toast notifications** — transient status messages

### Architecture

The TUI typically runs in a **separate worker process** that communicates with the backend server via RPC. This keeps the UI responsive even during heavy processing. State management uses a reactive framework so UI updates are automatic when data changes.

---

## 9. Supporting Infrastructure

### Event Bus

A typed pub/sub event system decouples components:

- Session events (created, updated, deleted)
- File watcher events (files changed on disk)
- Permission events (asked, replied)
- LSP events (server started, diagnostics received)

Events are validated against schemas before publishing, ensuring type safety across the system.

### Language Server Protocol (LSP) Integration

- Auto-discovers and launches language servers based on file type (TypeScript, Python, etc.)
- Provides the agent with hover information, diagnostics, go-to-definition, and symbol search
- Connection pooling — reuses server connections for files in the same project root
- Lazy initialization — servers only start when a relevant file is first accessed

### File Watcher

- Platform-native file watching (fsevents on macOS, inotify on Linux)
- Monitors the working directory and VCS metadata (e.g., `.git/`)
- Publishes change events to the event bus
- Configurable ignore patterns

### Plugin System

Plugins can extend the agent at multiple points:

- **Add custom tools** — new capabilities beyond the built-in set
- **Modify system prompts** — inject additional instructions
- **Transform messages** — alter conversation history before sending to the LLM
- **Hook into tool execution** — run logic before/after tool calls
- **Modify LLM parameters** — adjust temperature, headers, etc.

### MCP (Model Context Protocol)

MCP allows connecting external tool servers that expose additional capabilities. Tools from MCP servers are dynamically discovered and added to the tool registry, appearing alongside built-in tools.

### Configuration

Configuration is loaded hierarchically (lowest to highest precedence):

1. Remote organization defaults
2. Global user config (`~/.config/`)
3. Project-level config (in the repository)
4. Inline overrides (environment variables, CLI flags)

Configuration supports variable substitution (`{env:VAR_NAME}`) and is validated against a schema at load time.

---

## 10. Key Architectural Patterns

1. **Instance scoping** — all mutable state is bound to a directory-scoped instance, enabling multiple projects simultaneously.
2. **Lazy initialization** — heavy resources (LSP servers, provider connections) only start when first needed.
3. **Schema validation at boundaries** — every data boundary is validated: configuration, messages, tool inputs, events.
4. **Provider abstraction via transformation layers** — provider differences are handled through transformation pipelines rather than conditional branches.
5. **File-based persistence** — JSON files with file-level locking instead of a database, keeping the system simple and portable.
6. **Context management** — automatic compaction when conversation history exceeds context limits, with summarization preserving key information.
7. **Event-driven communication** — components are loosely coupled through a typed event bus rather than direct dependencies.

---

## Summary: What It Takes to Build a Coding Agent

At minimum, you need:

1. **An agentic loop** — call LLM → execute tools → feed results back → repeat.
2. **Tools** — file read/write/edit, shell execution, and search at minimum.
3. **A permission system** — you can't let an AI run arbitrary shell commands without guardrails.
4. **Provider integration** — streaming responses, tool calling protocol, token management.
5. **A system prompt** — instructions that make the LLM behave as a coding assistant.
6. **Session management** — persist conversations, handle context window limits.
7. **A terminal UI** — render streaming markdown, show tool execution status, handle user input.

The sophistication comes from: multi-provider support, LSP integration, plugin extensibility, project-aware context, and a polished terminal experience. But the core is always that loop.
