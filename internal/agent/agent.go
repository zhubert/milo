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
	// Model is the Claude model used by the agent.
	Model          = anthropic.ModelClaudeSonnet4_20250514
	defaultMaxToks = 8192
)

// ModelDisplayName returns a human-readable name for the current model.
func ModelDisplayName() string {
	return "Claude Sonnet 4"
}

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

// StreamChunk is a unit of output from the agent's streaming loop.
type StreamChunk struct {
	Type             ChunkType
	Text             string
	ToolName         string
	ToolID           string
	ToolInput        string
	Result           *tool.Result
	Err              error
	ParallelProgress *tool.ProgressUpdate   // For ChunkParallelProgress
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
	PermResp   chan PermissionResponse
}

const defaultWorkerCount = 4

// New creates a new Agent with the given client, registry, permission checker,
// working directory, and logger.
func New(client anthropic.Client, registry *tool.Registry, perms *permission.Checker, workDir string, logger *slog.Logger) *Agent {
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
		PermResp: make(chan PermissionResponse, 1),
	}
}

// ModelDisplayName returns a human-readable name for the current model.
func (a *Agent) ModelDisplayName() string {
	return ModelDisplayName()
}

// Permissions returns the permission checker for this agent.
func (a *Agent) Permissions() *permission.Checker {
	return a.perms
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
			Model:    Model,
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
					assistantBlocks = append(assistantBlocks,
						anthropic.NewToolUseBlock(currentToolID, json.RawMessage(currentToolInput), currentToolName),
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
			ch <- StreamChunk{Type: ChunkDone}
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
		tu      toolUseInfo
		t       tool.Tool
		allowed bool
	}
	statuses := make([]toolStatus, len(toolUseBlocks))

	for i, tu := range toolUseBlocks {
		a.logger.Info("tool execution start", "tool", tu.name, "tool_id", tu.id)
		a.logger.Debug("tool input", "tool", tu.name, "input", tu.input)

		ch <- StreamChunk{
			Type:      ChunkToolUse,
			ToolName:  tu.name,
			ToolID:    tu.id,
			ToolInput: tu.input,
		}

		t := a.registry.Lookup(tu.name)
		if t == nil {
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

		// Check permission.
		if !a.checkPermission(ctx, ch, tu.name, json.RawMessage(tu.input)) {
			if ctx.Err() != nil {
				return resultBlocks, true // Cancelled
			}
			a.logger.Warn("permission denied", "tool", tu.name)
			result := tool.Result{Output: "permission denied by user", IsError: true}
			resultBlocks = append(resultBlocks,
				anthropic.NewToolResultBlock(tu.id, result.Output, result.IsError),
			)
			a.detector.RecordToolCall(tu.name, tu.input, result.Output, result.IsError)
			ch <- StreamChunk{Type: ChunkToolResult, ToolName: tu.name, ToolID: tu.id, Result: &result}
			statuses[i] = toolStatus{tu: tu, t: t, allowed: false}
			continue
		}

		statuses[i] = toolStatus{tu: tu, t: t, allowed: true}
	}

	// Phase 2: Execute allowed tools in parallel.
	var allowedCalls []tool.ToolCall
	var allowedIndices []int

	for i, s := range statuses {
		if s.allowed && s.t != nil {
			allowedCalls = append(allowedCalls, tool.ToolCall{
				ID:    s.tu.id,
				Name:  s.tu.name,
				Input: json.RawMessage(s.tu.input),
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
