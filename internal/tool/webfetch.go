package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	webFetchTimeout     = 30 * time.Second
	maxWebFetchBodySize = 1024 * 1024 // 1MB
)

// WebFetchTool fetches content from a URL.
// It implements ParallelSafeTool since it only reads external data.
type WebFetchTool struct{}

// IsParallelSafe returns true since fetch operations don't modify local state.
func (t *WebFetchTool) IsParallelSafe() bool { return true }

type webFetchInput struct {
	URL string `json:"url"`
}

func (t *WebFetchTool) Name() string { return "web_fetch" }

func (t *WebFetchTool) Description() string {
	return "Fetch content from a URL. Returns the page content with HTML tags stripped " +
		"for readability. Useful for reading documentation, API references, and web pages. " +
		"Content is limited to 1MB. Timeout is 30 seconds."
}

func (t *WebFetchTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to fetch content from (must include http:// or https://)",
			},
		},
		Required: []string{"url"},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in webFetchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing web_fetch input: %w", err)
	}

	if in.URL == "" {
		return Result{Output: "url is required", IsError: true}, nil
	}

	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		return Result{Output: "url must start with http:// or https://", IsError: true}, nil
	}

	client := &http.Client{
		Timeout: webFetchTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return Result{Output: fmt.Sprintf("error creating request: %s", err), IsError: true}, nil
	}

	req.Header.Set("User-Agent", "Milo/1.0 (Coding Agent)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return Result{Output: fmt.Sprintf("error fetching URL: %s", err), IsError: true}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Result{Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status), IsError: true}, nil
	}

	limitedReader := io.LimitReader(resp.Body, maxWebFetchBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return Result{Output: fmt.Sprintf("error reading response: %s", err), IsError: true}, nil
	}

	content := string(body)
	contentType := resp.Header.Get("Content-Type")

	// Strip HTML if it looks like HTML content.
	if strings.Contains(contentType, "text/html") || strings.Contains(content, "<html") {
		content = stripHTML(content)
	}

	// Truncate if still too long after processing.
	const maxOutputLen = 100000
	if len(content) > maxOutputLen {
		content = content[:maxOutputLen] + "\n\n(content truncated)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "URL: %s\n", in.URL)
	fmt.Fprintf(&b, "Status: %d\n", resp.StatusCode)
	fmt.Fprintf(&b, "Content-Type: %s\n", contentType)
	fmt.Fprintln(&b, "---")
	fmt.Fprintln(&b, content)

	return Result{Output: b.String()}, nil
}

// stripHTML removes HTML tags and cleans up the content for readability.
func stripHTML(html string) string {
	// Remove script and style elements entirely.
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")

	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Remove HTML comments.
	commentRe := regexp.MustCompile(`(?s)<!--.*?-->`)
	html = commentRe.ReplaceAllString(html, "")

	// Replace common block elements with newlines.
	blockRe := regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr|br|hr)[^>]*>`)
	html = blockRe.ReplaceAllString(html, "\n")

	// Handle br tags.
	brRe := regexp.MustCompile(`(?i)<br[^>]*/?>`)
	html = brRe.ReplaceAllString(html, "\n")

	// Remove all remaining HTML tags.
	tagRe := regexp.MustCompile(`<[^>]+>`)
	html = tagRe.ReplaceAllString(html, "")

	// Decode common HTML entities.
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")
	html = strings.ReplaceAll(html, "&apos;", "'")

	// Collapse multiple whitespace and newlines.
	spaceRe := regexp.MustCompile(`[ \t]+`)
	html = spaceRe.ReplaceAllString(html, " ")

	newlineRe := regexp.MustCompile(`\n{3,}`)
	html = newlineRe.ReplaceAllString(html, "\n\n")

	// Trim each line.
	lines := strings.Split(html, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}
