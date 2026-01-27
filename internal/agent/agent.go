package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/zhubert/looper/internal/permission"
	"github.com/zhubert/looper/internal/tool"
)

const (
	defaultModel   = anthropic.ModelClaude4Sonnet20250514
	defaultMaxToks = 8192
)

// ChunkType identifies the kind of stream chunk.
type ChunkType int

const (
	ChunkText ChunkType = iota
	ChunkToolUse
	ChunkToolResult
	ChunkPermissionRequest
	ChunkDone
	ChunkError
)

// StreamChunk is a unit of output from the agent's streaming loop.
type StreamChunk struct {
	Type      ChunkType
	Text      string
	ToolName  string
	ToolID    string
	ToolInput string
	Result    *tool.Result
	Err       error
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
	workDir    string
	PermResp   chan PermissionResponse
}

// New creates a new Agent with the given client, registry, permission checker,
// and working directory.
func New(client anthropic.Client, registry *tool.Registry, perms *permission.Checker, workDir string) *Agent {
	return &Agent{
		client:   client,
		registry: registry,
		perms:    perms,
		conv:     NewConversation(),
		workDir:  workDir,
		PermResp: make(chan PermissionResponse, 1),
	}
}

// SendMessage starts the agentic loop for the given user message.
// It returns a channel that emits StreamChunks as the response is generated.
// The channel is closed when the loop completes.
func (a *Agent) SendMessage(ctx context.Context, userMsg string) <-chan StreamChunk {
	ch := make(chan StreamChunk, 64)

	a.conv.AddUserMessage(userMsg)

	go func() {
		defer close(ch)
		a.loop(ctx, ch)
	}()

	return ch
}

func (a *Agent) loop(ctx context.Context, ch chan<- StreamChunk) {
	for {
		if ctx.Err() != nil {
			return
		}

		systemPrompt := BuildSystemPrompt(a.workDir, a.registry)

		stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:    defaultModel,
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

		// Execute tools and collect results.
		var resultBlocks []anthropic.ContentBlockParamUnion
		for _, tu := range toolUseBlocks {
			ch <- StreamChunk{
				Type:      ChunkToolUse,
				ToolName:  tu.name,
				ToolID:    tu.id,
				ToolInput: tu.input,
			}

			t := a.registry.Lookup(tu.name)
			if t == nil {
				result := tool.Result{Output: fmt.Sprintf("unknown tool: %s", tu.name), IsError: true}
				resultBlocks = append(resultBlocks,
					anthropic.NewToolResultBlock(tu.id, result.Output, result.IsError),
				)
				ch <- StreamChunk{Type: ChunkToolResult, ToolName: tu.name, ToolID: tu.id, Result: &result}
				continue
			}

			// Check permission before executing.
			if !a.checkPermission(ctx, ch, tu.name) {
				result := tool.Result{Output: "permission denied by user", IsError: true}
				resultBlocks = append(resultBlocks,
					anthropic.NewToolResultBlock(tu.id, result.Output, result.IsError),
				)
				ch <- StreamChunk{Type: ChunkToolResult, ToolName: tu.name, ToolID: tu.id, Result: &result}
				continue
			}

			result, err := t.Execute(ctx, json.RawMessage(tu.input))
			if err != nil {
				log.Printf("tool execution error (%s): %v", tu.name, err)
				result = tool.Result{Output: fmt.Sprintf("tool execution error: %s", err), IsError: true}
			}

			resultBlocks = append(resultBlocks,
				anthropic.NewToolResultBlock(tu.id, result.Output, result.IsError),
			)
			ch <- StreamChunk{Type: ChunkToolResult, ToolName: tu.name, ToolID: tu.id, Result: &result}
		}

		a.conv.AddToolResult(resultBlocks...)
		// Loop continues — Claude will see the tool results.
	}
}

// checkPermission evaluates the permission for a tool and, if needed,
// sends a permission request and blocks until the user responds.
// Returns true if the tool is allowed to execute.
func (a *Agent) checkPermission(ctx context.Context, ch chan<- StreamChunk, toolName string) bool {
	action := a.perms.Check(toolName)
	switch action {
	case permission.Allow:
		return true
	case permission.Deny:
		return false
	case permission.Ask:
		ch <- StreamChunk{Type: ChunkPermissionRequest, ToolName: toolName}

		select {
		case resp := <-a.PermResp:
			switch resp {
			case PermissionGranted:
				return true
			case PermissionGrantedAlways:
				a.perms.AllowAlways(toolName)
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
