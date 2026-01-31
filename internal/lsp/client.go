package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRequestTimeout = 30 * time.Second
)

// Client manages communication with a language server over JSON-RPC 2.0.
type Client struct {
	config  *ServerConfig
	rootDir string

	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	nextID   atomic.Int64
	pending  map[int64]chan *jsonRPCResponse
	pendingM sync.Mutex

	initialized bool
	closeOnce   sync.Once
	done        chan struct{}
}

// jsonRPCRequest represents a JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// jsonRPCResponse represents a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// jsonRPCError represents a JSON-RPC 2.0 error.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *jsonRPCError) Error() string {
	return fmt.Sprintf("LSP error %d: %s", e.Code, e.Message)
}

// NewClient creates a new LSP client for the given server configuration.
func NewClient(config *ServerConfig, rootDir string) *Client {
	return &Client{
		config:  config,
		rootDir: rootDir,
		pending: make(map[int64]chan *jsonRPCResponse),
		done:    make(chan struct{}),
	}
}

// Start launches the language server and performs initialization.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	if len(c.config.Command) == 0 {
		return fmt.Errorf("no command configured for %s", c.config.Name)
	}

	// Start the language server process
	cmd := exec.CommandContext(ctx, c.config.Command[0], c.config.Command[1:]...)
	cmd.Dir = c.rootDir
	cmd.Env = append(os.Environ(), "PWD="+c.rootDir)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if cerr := stdin.Close(); cerr != nil {
			return fmt.Errorf("closing stdin after stdout error: %w (original: %w)", cerr, err)
		}
		return fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		if cerr := stdin.Close(); cerr != nil {
			return fmt.Errorf("closing stdin after start error: %w (original: %w)", cerr, err)
		}
		if cerr := stdout.Close(); cerr != nil {
			return fmt.Errorf("closing stdout after start error: %w (original: %w)", cerr, err)
		}
		return fmt.Errorf("starting %s: %w", c.config.Name, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout

	// Start reading responses in background
	go c.readResponses()

	// Perform LSP initialize handshake
	if err := c.initialize(ctx); err != nil {
		_ = c.close() // Best effort cleanup on initialization failure
		return fmt.Errorf("initializing %s: %w", c.config.Name, err)
	}

	c.initialized = true
	return nil
}

// initialize performs the LSP initialize/initialized handshake.
func (c *Client) initialize(ctx context.Context) error {
	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   FileToURI(c.rootDir),
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Hover: HoverClientCapabilities{
					ContentFormat: []string{"markdown", "plaintext"},
				},
			},
		},
	}

	var result InitializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize request: %w", err)
	}

	// Send initialized notification
	if err := c.notify(ctx, "initialized", struct{}{}); err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}

	return nil
}

// Definition returns the location(s) where the symbol at the given position is defined.
func (c *Client) Definition(ctx context.Context, file string, line, column int) ([]Location, error) {
	if err := c.ensureStarted(ctx); err != nil {
		return nil, err
	}

	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: FileToURI(file)},
		Position:     ToLSPPosition(line, column),
	}

	var result json.RawMessage
	if err := c.call(ctx, "textDocument/definition", params, &result); err != nil {
		return nil, err
	}

	return c.parseLocationResult(result)
}

// References returns all locations that reference the symbol at the given position.
func (c *Client) References(ctx context.Context, file string, line, column int) ([]Location, error) {
	if err := c.ensureStarted(ctx); err != nil {
		return nil, err
	}

	params := ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: FileToURI(file)},
			Position:     ToLSPPosition(line, column),
		},
		Context: ReferenceContext{
			IncludeDeclaration: true,
		},
	}

	var result json.RawMessage
	if err := c.call(ctx, "textDocument/references", params, &result); err != nil {
		return nil, err
	}

	return c.parseLocationResult(result)
}

// Hover returns hover information for the symbol at the given position.
func (c *Client) Hover(ctx context.Context, file string, line, column int) (*Hover, error) {
	if err := c.ensureStarted(ctx); err != nil {
		return nil, err
	}

	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: FileToURI(file)},
		Position:     ToLSPPosition(line, column),
	}

	var result *Hover
	if err := c.call(ctx, "textDocument/hover", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// WorkspaceSymbols searches for symbols matching the given query.
func (c *Client) WorkspaceSymbols(ctx context.Context, query string) ([]SymbolInformation, error) {
	if err := c.ensureStarted(ctx); err != nil {
		return nil, err
	}

	params := WorkspaceSymbolParams{
		Query: query,
	}

	var result []SymbolInformation
	if err := c.call(ctx, "workspace/symbol", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Close shuts down the language server.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.close()
}

func (c *Client) close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		close(c.done)

		if c.stdin != nil {
			// Try to send shutdown request, but don't wait long
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = c.call(ctx, "shutdown", nil, nil)
			_ = c.notify(ctx, "exit", nil)
			cancel()

			if err := c.stdin.Close(); err != nil {
				closeErr = fmt.Errorf("closing stdin: %w", err)
			}
		}

		if c.cmd != nil && c.cmd.Process != nil {
			// Give it a moment to exit gracefully
			done := make(chan struct{})
			go func() {
				_ = c.cmd.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = c.cmd.Process.Kill()
			}
		}
	})
	return closeErr
}

// ensureStarted ensures the client is started.
func (c *Client) ensureStarted(ctx context.Context) error {
	c.mu.Lock()
	initialized := c.initialized
	c.mu.Unlock()

	if !initialized {
		return c.Start(ctx)
	}
	return nil
}

// call makes a JSON-RPC request and waits for the response.
func (c *Client) call(ctx context.Context, method string, params, result any) error {
	id := c.nextID.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	respCh := make(chan *jsonRPCResponse, 1)
	c.pendingM.Lock()
	c.pending[id] = respCh
	c.pendingM.Unlock()

	defer func() {
		c.pendingM.Lock()
		delete(c.pending, id)
		c.pendingM.Unlock()
	}()

	if err := c.send(req); err != nil {
		return err
	}

	// Wait for response with timeout
	timeout := defaultRequestTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && resp.Result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("unmarshaling result: %w", err)
			}
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("request timed out after %v", timeout)
	case <-c.done:
		return fmt.Errorf("client closed")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// notify sends a JSON-RPC notification (no response expected).
func (c *Client) notify(ctx context.Context, method string, params any) error {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.send(req)
}

// send writes a JSON-RPC message to the server.
func (c *Client) send(req jsonRPCRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin == nil {
		return fmt.Errorf("client not started")
	}

	_, err = io.WriteString(c.stdin, msg)
	return err
}

// readResponses reads and dispatches responses from the server.
func (c *Client) readResponses() {
	reader := bufio.NewReader(c.stdout)

	for {
		select {
		case <-c.done:
			return
		default:
		}

		// Read headers
		contentLength := 0
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				contentLength, _ = strconv.Atoi(lengthStr)
			}
		}

		if contentLength == 0 {
			continue
		}

		// Read body
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return
		}

		// Parse response
		var resp jsonRPCResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		// Dispatch to waiting caller
		c.pendingM.Lock()
		if ch, ok := c.pending[resp.ID]; ok {
			select {
			case ch <- &resp:
			default:
			}
		}
		c.pendingM.Unlock()
	}
}

// parseLocationResult handles the different formats LSP servers may return for locations.
func (c *Client) parseLocationResult(raw json.RawMessage) ([]Location, error) {
	if raw == nil || string(raw) == "null" {
		return nil, nil
	}

	// Try as single location first
	var single Location
	if err := json.Unmarshal(raw, &single); err == nil && single.URI != "" {
		return []Location{single}, nil
	}

	// Try as array of locations
	var multiple []Location
	if err := json.Unmarshal(raw, &multiple); err == nil {
		return multiple, nil
	}

	// Try as LocationLink array (some servers return this)
	var links []struct {
		TargetURI   string `json:"targetUri"`
		TargetRange Range  `json:"targetRange"`
	}
	if err := json.Unmarshal(raw, &links); err == nil {
		locations := make([]Location, 0, len(links))
		for _, link := range links {
			locations = append(locations, Location{
				URI:   link.TargetURI,
				Range: link.TargetRange,
			})
		}
		return locations, nil
	}

	return nil, nil
}

// IsRunning returns whether the language server process is running.
func (c *Client) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cmd != nil && c.cmd.ProcessState == nil
}

// ServerName returns the name of the language server.
func (c *Client) ServerName() string {
	return c.config.Name
}
