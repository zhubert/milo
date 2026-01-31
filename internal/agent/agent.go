package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	ctxmgr "github.com/zhubert/milo/internal/context"
	"github.com/zhubert/milo/internal/loopdetector"
	"github.com/zhubert/milo/internal/permission"
	"github.com/zhubert/milo/internal/tool"
)

const (
	// DefaultModel is the Claude model used when none is specified.
	DefaultModel   = anthropic.ModelClaudeSonnet4_20250514
	defaultMaxToks = 8192
)

// ChunkType identifies the kind of stream chunk.
type ChunkType int

const (
	ChunkText ChunkType = iota
	ChunkToolUse
	ChunkToolResult
	ChunkPermissionRequest
	ChunkParallelProgress
	ChunkContextCompacted
	ChunkDone
	ChunkError
)

// Usage tracks token consumption for a single API turn.
type Usage struct {
	Model        string
	InputTokens  int64
	OutputTokens int64
}

// StreamChunk is a unit of output from the agent's streaming loop.
type StreamChunk struct {
	Type             ChunkType
	Text             string
	ToolName         string
	ToolID           string
	ToolInput        string
	Result           *tool.Result
	Err              error
	ParallelProgress *tool.ProgressUpdate     // For ChunkParallelProgress
	Usage            *Usage                   // For ChunkDone - token usage for this turn
	CompactionInfo   *ctxmgr.CompactionResult // For ChunkContextCompacted
}

// PermissionResponse is the user's answer to a permission request.
type PermissionResponse int

const (
	PermissionGranted      PermissionResponse = iota // Allow this once
	PermissionDenied                                  // Deny this once
	PermissionGrantedAlways                           // Allow always for this session
)

// Agent is the core agentic loop that sends messages to Claude,
// streams the response, executes tools, and loops until done.
type Agent struct {
	client     anthropic.Client
	registry   *tool.Registry
	perms      *permission.Checker
	conv       *Conversation
	detector   *loopdetector.Detector
	executor   *tool.ToolExecutor
	ctxMgr     *ctxmgr.Manager
	workDir    string
	logger     *slog.Logger
	model      anthropic.Model
	PermResp   chan PermissionResponse
}

const defaultWorkerCount = 4

// New creates a new Agent with the given client, registry, permission checker,
// working directory, logger, and model.
func New(client anthropic.Client, registry *tool.Registry, perms *permission.Checker, workDir string, logger *slog.Logger, model anthropic.Model) *Agent {
	// Create summarizer using the same client (will use Haiku model)
	summarizer := ctxmgr.NewHaikuSummarizer(client)

	return &Agent{
		client:   client,
		registry: registry,
		perms:    perms,
		conv:     NewConversation(),
		detector: loopdetector.NewWithDefaults(),
		executor: tool.NewToolExecutor(registry, defaultWorkerCount),
		ctxMgr:   ctxmgr.NewManagerWithDefaults(summarizer),
		workDir:  workDir,
		logger:   logger,
		model:    model,
		PermResp: make(chan PermissionResponse, 1),
	}
}

// ModelDisplayName returns a human-readable name for the current model.
func (a *Agent) ModelDisplayName() string {
	return modelDisplayName(a.model)
}

// modelDisplayName returns a human-readable name for the given model.
func modelDisplayName(model anthropic.Model) string {
	switch model {
	case anthropic.ModelClaudeOpus4_5_20251101, anthropic.ModelClaudeOpus4_5:
		return "Claude Opus 4.5"
	case anthropic.ModelClaudeOpus4_0, anthropic.ModelClaudeOpus4_20250514, anthropic.ModelClaude4Opus20250514:
		return "Claude Opus 4"
	case anthropic.ModelClaudeOpus4_1_20250805:
		return "Claude Opus 4.1"
	case anthropic.ModelClaudeSonnet4_5, anthropic.ModelClaudeSonnet4_5_20250929:
		return "Claude Sonnet 4.5"
	case anthropic.ModelClaudeSonnet4_20250514, anthropic.ModelClaudeSonnet4_0, anthropic.ModelClaude4Sonnet20250514:
		return "Claude Sonnet 4"
	case anthropic.ModelClaude3_7SonnetLatest, anthropic.ModelClaude3_7Sonnet20250219:
		return "Claude 3.7 Sonnet"
	case anthropic.ModelClaude3_5HaikuLatest, anthropic.ModelClaude3_5Haiku20241022:
		return "Claude 3.5 Haiku"
	case anthropic.ModelClaudeHaiku4_5, anthropic.ModelClaudeHaiku4_5_20251001:
		return "Claude Haiku 4.5"
	case anthropic.ModelClaude3OpusLatest, anthropic.ModelClaude_3_Opus_20240229:
		return "Claude 3 Opus"
	case anthropic.ModelClaude_3_Haiku_20240307:
		return "Claude 3 Haiku"
	default:
		return string(model)
	}
}

// Permissions returns the permission checker for this agent.
func (a *Agent) Permissions() *permission.Checker {
	return a.perms
}

// Messages returns a copy of the conversation messages.
func (a *Agent) Messages() []anthropic.MessageParam {
	return a.conv.Messages()
}

// SetMessages replaces the conversation messages with the provided slice.
// This is used to restore a conversation from a saved session.
func (a *Agent) SetMessages(messages []anthropic.MessageParam) {
	a.conv.SetMessages(messages)
}

// Model returns the current model identifier.
func (a *Agent) Model() string {
	return string(a.model)
}

// SetModel changes the model used for subsequent messages.
func (a *Agent) SetModel(model string) {
	a.model = anthropic.Model(model)
}

// ModelOption represents an available model choice.
type ModelOption struct {
	ID          string // The model identifier string
	DisplayName string // Human-readable name
}

// AvailableModels returns the list of supported models.
func AvailableModels() []ModelOption {
	return []ModelOption{
		{ID: string(anthropic.ModelClaudeSonnet4_20250514), DisplayName: "Claude Sonnet 4"},
		{ID: string(anthropic.ModelClaudeSonnet4_5), DisplayName: "Claude Sonnet 4.5"},
		{ID: string(anthropic.ModelClaudeOpus4_5), DisplayName: "Claude Opus 4.5"},
		{ID: string(anthropic.ModelClaudeOpus4_0), DisplayName: "Claude Opus 4"},
		{ID: string(anthropic.ModelClaudeHaiku4_5), DisplayName: "Claude Haiku 4.5"},
		{ID: string(anthropic.ModelClaude3_5HaikuLatest), DisplayName: "Claude 3.5 Haiku"},
	}
}

// TokenCount returns the current estimated token count for the conversation.
func (a *Agent) TokenCount() int {
	return a.conv.TokenCount()
}

// ContextLimits returns the context window limits configuration.
func (a *Agent) ContextLimits() (available, used int) {
	return a.ctxMgr.Limits().AvailableTokens(), a.conv.TokenCount()
}

// SendMessage starts the agentic loop for the given user message.
// It returns a channel that emits StreamChunks as the response is generated.
// The channel is closed when the loop completes.
func (a *Agent) SendMessage(ctx context.Context, userMsg string) <-chan StreamChunk {
	ch := make(chan StreamChunk, 64)

	a.conv.AddUserMessage(userMsg)
	a.detector.Reset() // Reset doom loop detector for new request

	go func() {
		defer close(ch)
		a.loop(ctx, ch)
	}()

	return ch
}

func (a *Agent) loop(ctx context.Context, ch chan<- StreamChunk) {
	a.logger.Info("agent loop started")
	defer a.logger.Info("agent loop ended")

	// Accumulate token usage across all API calls in this agent turn.
	var totalInputTokens, totalOutputTokens int64

	for {
		if ctx.Err() != nil {
			a.logger.Info("context cancelled, stopping loop")
			return
		}

		// Record and check for doom loop at start of each iteration
		a.detector.RecordIteration()
		if detection := a.detector.Check(); detection.Detected {
			a.logger.Warn("doom loop detected", "reason", detection.Reason, "iterations", a.detector.Iterations())
			ch <- StreamChunk{
				Type: ChunkError,
				Err:  fmt.Errorf("doom loop detected: %s", detection.Reason),
			}
			return
		}

		// Check if context window needs compaction
		if a.ctxMgr.NeedsCompaction(a.conv.Messages()) {
			a.logger.Info("context window compaction triggered",
				"tokens", a.conv.TokenCount(),
				"threshold", a.ctxMgr.Limits().SummarizationTrigger())

			result, err := a.ctxMgr.Compact(ctx, a.conv.Messages())
			if err != nil {
				a.logger.Error("context compaction failed", "error", err)
				// Continue anyway - the API call might fail due to context limits
			} else {
				a.conv.SetMessages(result.Messages)
				a.logger.Info("context window compacted",
					"original_tokens", result.OriginalTokens,
					"compacted_tokens", result.CompactedTokens,
					"summary_added", result.SummaryAdded)

				ch <- StreamChunk{
					Type:           ChunkContextCompacted,
					CompactionInfo: result,
				}
			}
		}

		systemPrompt := BuildSystemPrompt(a.workDir, a.registry)

		stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     a.model,
			MaxTokens: defaultMaxToks,
			System: []anthropic.TextBlockParam{
				{Text: systemPrompt},
			},
			Messages: a.conv.Messages(),
			Tools:    a.registry.ToolParams(),
		})

		var assistantBlocks []anthropic.ContentBlockParamUnion
		var toolUseBlocks []toolUseInfo
		var currentText string
		var currentToolID string
		var currentToolName string
		var currentToolInput string

		for stream.Next() {
			event := stream.Current()

			switch event.Type {
			case "content_block_start":
				cb := event.ContentBlock
				switch cb.Type {
				case "text":
					currentText = ""
				case "tool_use":
					currentToolID = cb.ID
					currentToolName = cb.Name
					currentToolInput = ""
				}

			case "content_block_delta":
				delta := event.Delta
				switch delta.Type {
				case "text_delta":
					currentText += delta.Text
					ch <- StreamChunk{Type: ChunkText, Text: delta.Text}
				case "input_json_delta":
					currentToolInput += delta.PartialJSON
				}

			case "content_block_stop":
				if currentToolName != "" {
					// Ensure empty input is valid JSON for tools with no required params.
					inputJSON := currentToolInput
					if inputJSON == "" {
						inputJSON = "{}"
					}
					assistantBlocks = append(assistantBlocks,
						anthropic.NewToolUseBlock(currentToolID, json.RawMessage(inputJSON), currentToolName),
					)
					toolUseBlocks = append(toolUseBlocks, toolUseInfo{
						id:    currentToolID,
						name:  currentToolName,
						input: currentToolInput,
					})
					currentToolName = ""
					currentToolID = ""
					currentToolInput = ""
				} else if currentText != "" {
					assistantBlocks = append(assistantBlocks,
						anthropic.NewTextBlock(currentText),
					)
					currentText = ""
				}

			case "message_delta":
				// Message is ending; stop_reason is in event.Delta.StopReason.
				// Capture usage data if available.
				if event.Usage.InputTokens > 0 || event.Usage.OutputTokens > 0 {
					totalInputTokens += event.Usage.InputTokens
					totalOutputTokens += event.Usage.OutputTokens
				}
			}
		}

		if err := stream.Err(); err != nil {
			if ctx.Err() != nil {
				return
			}
			a.logger.Error("stream error", "error", err)
			ch <- StreamChunk{Type: ChunkError, Err: fmt.Errorf("stream error: %w", err)}
			return
		}

		// Record the assistant's response.
		if len(assistantBlocks) > 0 {
			a.conv.AddAssistantMessage(assistantBlocks...)
		}

		// If there are no tool use blocks, we're done.
		if len(toolUseBlocks) == 0 {
			ch <- StreamChunk{
				Type: ChunkDone,
				Usage: &Usage{
					Model:        string(a.model),
					InputTokens:  totalInputTokens,
					OutputTokens: totalOutputTokens,
				},
			}
			return
		}

		// Execute tools - check permissions first, then execute in parallel where safe.
		resultBlocks, cancelled := a.executeTools(ctx, ch, toolUseBlocks)
		if cancelled {
			return
		}

		// Check for doom loop after processing all tool calls
		if detection := a.detector.Check(); detection.Detected {
			a.logger.Warn("doom loop detected after tool execution", "reason", detection.Reason, "iterations", a.detector.Iterations())
			ch <- StreamChunk{
				Type: ChunkError,
				Err:  fmt.Errorf("doom loop detected: %s", detection.Reason),
			}
			return
		}

		a.conv.AddToolResult(resultBlocks...)
		// Loop continues — Claude will see the tool results.
	}
}

// checkPermission evaluates the permission for a tool and, if needed,
// sends a permission request and blocks until the user responds.
// Returns true if the tool is allowed to execute.
func (a *Agent) checkPermission(ctx context.Context, ch chan<- StreamChunk, toolName string, toolInput json.RawMessage) bool {
	action := a.perms.Check(toolName, toolInput)
	switch action {
	case permission.Allow:
		return true
	case permission.Deny:
		return false
	case permission.Ask:
		ch <- StreamChunk{Type: ChunkPermissionRequest, ToolName: toolName, ToolInput: string(toolInput)}

		select {
		case resp := <-a.PermResp:
			switch resp {
			case PermissionGranted:
				return true
			case PermissionGrantedAlways:
				if err := a.perms.AllowAlways(toolName, toolInput); err != nil {
					a.logger.Warn("failed to persist permission", "error", err)
				}
				return true
			default:
				return false
			}
		case <-ctx.Done():
			return false
		}
	}
	return false
}

type toolUseInfo struct {
	id    string
	name  string
	input string
}

// executeTools handles permission checks and parallel tool execution.
// Returns the result blocks and whether execution was cancelled.
func (a *Agent) executeTools(ctx context.Context, ch chan<- StreamChunk, toolUseBlocks []toolUseInfo) ([]anthropic.ContentBlockParamUnion, bool) {
	var resultBlocks []anthropic.ContentBlockParamUnion

	// Phase 1: Check permissions for all tools (must be sequential for user interaction).
	// Separate tools into: allowed, denied, and unknown.
	type toolStatus struct {
		tu              toolUseInfo
		t               tool.Tool
		normalizedInput string // Input after normalization (e.g., cd prefix stripped)
		allowed         bool
	}
	statuses := make([]toolStatus, len(toolUseBlocks))

	for i, tu := range toolUseBlocks {
		a.logger.Info("tool execution start", "tool", tu.name, "tool_id", tu.id)
		a.logger.Debug("tool input", "tool", tu.name, "input", tu.input)

		t := a.registry.Lookup(tu.name)
		if t == nil {
			ch <- StreamChunk{
				Type:      ChunkToolUse,
				ToolName:  tu.name,
				ToolID:    tu.id,
				ToolInput: tu.input,
			}
			a.logger.Warn("unknown tool", "tool", tu.name)
			result := tool.Result{Output: fmt.Sprintf("unknown tool: %s", tu.name), IsError: true}
			resultBlocks = append(resultBlocks,
				anthropic.NewToolResultBlock(tu.id, result.Output, result.IsError),
			)
			a.detector.RecordToolCall(tu.name, tu.input, result.Output, result.IsError)
			ch <- StreamChunk{Type: ChunkToolResult, ToolName: tu.name, ToolID: tu.id, Result: &result}
			statuses[i] = toolStatus{tu: tu, t: nil, allowed: false}
			continue
		}

		// Normalize input if the tool supports it (e.g., bash strips "cd workdir &&").
		normalizedInput := tu.input
		if normalizer, ok := t.(tool.InputNormalizer); ok {
			normalizedInput = string(normalizer.NormalizeInput(json.RawMessage(tu.input)))
		}

		ch <- StreamChunk{
			Type:      ChunkToolUse,
			ToolName:  tu.name,
			ToolID:    tu.id,
			ToolInput: normalizedInput,
		}

		// Check permission using normalized input.
		if !a.checkPermission(ctx, ch, tu.name, json.RawMessage(normalizedInput)) {
			if ctx.Err() != nil {
				return resultBlocks, true // Cancelled
			}
			a.logger.Warn("permission denied", "tool", tu.name)
			result := tool.Result{Output: "permission denied by user", IsError: true}
			resultBlocks = append(resultBlocks,
				anthropic.NewToolResultBlock(tu.id, result.Output, result.IsError),
			)
			a.detector.RecordToolCall(tu.name, normalizedInput, result.Output, result.IsError)
			ch <- StreamChunk{Type: ChunkToolResult, ToolName: tu.name, ToolID: tu.id, Result: &result}
			statuses[i] = toolStatus{tu: tu, t: t, normalizedInput: normalizedInput, allowed: false}
			continue
		}

		statuses[i] = toolStatus{tu: tu, t: t, normalizedInput: normalizedInput, allowed: true}
	}

	// Phase 2: Execute allowed tools in parallel.
	var allowedCalls []tool.ToolCall
	var allowedIndices []int

	for i, s := range statuses {
		if s.allowed && s.t != nil {
			allowedCalls = append(allowedCalls, tool.ToolCall{
				ID:    s.tu.id,
				Name:  s.tu.name,
				Input: json.RawMessage(s.normalizedInput),
			})
			allowedIndices = append(allowedIndices, i)
		}
	}

	if len(allowedCalls) == 0 {
		return resultBlocks, false
	}

	// Set up progress channel.
	progressCh := make(chan tool.ProgressUpdate, len(allowedCalls)*2)
	go func() {
		for update := range progressCh {
			ch <- StreamChunk{
				Type:             ChunkParallelProgress,
				ParallelProgress: &update,
			}
		}
	}()

	// Execute tools in parallel.
	results, err := a.executor.ExecuteTools(ctx, allowedCalls, progressCh)
	close(progressCh)

	if err != nil {
		a.logger.Error("parallel execution error", "error", err)
	}

	// Process results in original order.
	for i, taskResult := range results {
		originalIdx := allowedIndices[i]
		tu := statuses[originalIdx].tu

		result := taskResult.Result
		if taskResult.Err != nil {
			a.logger.Error("tool execution error", "tool", tu.name, "error", taskResult.Err)
			result = tool.Result{Output: fmt.Sprintf("tool execution error: %s", taskResult.Err), IsError: true}
		}

		if result.IsError {
			a.logger.Warn("tool returned error result", "tool", tu.name, "output", result.Output)
		}

		resultBlocks = append(resultBlocks,
			anthropic.NewToolResultBlock(tu.id, result.Output, result.IsError),
		)
		a.detector.RecordToolCall(tu.name, tu.input, result.Output, result.IsError)
		ch <- StreamChunk{Type: ChunkToolResult, ToolName: tu.name, ToolID: tu.id, Result: &result}
	}

	return resultBlocks, false
}
