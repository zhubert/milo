package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	webSearchTimeout  = 30 * time.Second
	maxSearchResults  = 10
	duckDuckGoLiteURL = "https://lite.duckduckgo.com/lite/"
)

// WebSearchTool searches the web using DuckDuckGo.
// It implements ParallelSafeTool since it only reads external data.
type WebSearchTool struct{}

// IsParallelSafe returns true since search operations don't modify local state.
func (t *WebSearchTool) IsParallelSafe() bool { return true }

type webSearchInput struct {
	Query string `json:"query"`
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Search the web using DuckDuckGo. Returns search results with titles, URLs, and snippets. " +
		"Useful for finding documentation, troubleshooting errors, researching libraries, and getting current information. " +
		"Returns up to 10 results."
}

func (t *WebSearchTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
		},
		Required: []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in webSearchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing web_search input: %w", err)
	}

	if in.Query == "" {
		return Result{Output: "query is required", IsError: true}, nil
	}

	results, err := searchDuckDuckGo(ctx, in.Query)
	if err != nil {
		return Result{Output: fmt.Sprintf("search error: %s", err), IsError: true}, nil
	}

	if len(results) == 0 {
		return Result{Output: "no results found"}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Search results for: %s\n", in.Query)
	fmt.Fprintln(&b, strings.Repeat("=", 50))

	for i, r := range results {
		fmt.Fprintf(&b, "\n%d. %s\n", i+1, r.Title)
		fmt.Fprintf(&b, "   URL: %s\n", r.URL)
		if r.Snippet != "" {
			fmt.Fprintf(&b, "   %s\n", r.Snippet)
		}
	}

	return Result{Output: b.String()}, nil
}

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

func searchDuckDuckGo(ctx context.Context, query string) ([]searchResult, error) {
	client := &http.Client{
		Timeout: webSearchTimeout,
	}

	// Build form data for POST request.
	form := url.Values{}
	form.Set("q", query)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, duckDuckGoLiteURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Milo/1.0 (Coding Agent)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return parseDuckDuckGoLite(string(body))
}

// parseDuckDuckGoLite extracts search results from DuckDuckGo Lite HTML.
func parseDuckDuckGoLite(html string) ([]searchResult, error) {
	var results []searchResult

	// DuckDuckGo Lite uses a simple table format.
	// Look for result links which have class="result-link".
	linkRe := regexp.MustCompile(`<a[^>]+class="result-link"[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
	linkMatches := linkRe.FindAllStringSubmatch(html, maxSearchResults*2)

	// Also look for result snippets in the next table cell.
	snippetRe := regexp.MustCompile(`<td[^>]*class="result-snippet"[^>]*>([^<]+(?:<[^>]+>[^<]*)*)</td>`)
	snippetMatches := snippetRe.FindAllStringSubmatch(html, maxSearchResults*2)

	// Alternative pattern for links (some results use different format).
	altLinkRe := regexp.MustCompile(`<a[^>]+rel="nofollow"[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
	if len(linkMatches) == 0 {
		linkMatches = altLinkRe.FindAllStringSubmatch(html, maxSearchResults*2)
	}

	// Extract results.
	for i, match := range linkMatches {
		if len(match) < 3 {
			continue
		}

		linkURL := match[1]
		title := strings.TrimSpace(match[2])

		// Skip DuckDuckGo internal links.
		if strings.Contains(linkURL, "duckduckgo.com") {
			continue
		}

		// Decode HTML entities in title.
		title = decodeHTMLEntities(title)

		result := searchResult{
			Title: title,
			URL:   linkURL,
		}

		// Try to get corresponding snippet.
		if i < len(snippetMatches) && len(snippetMatches[i]) > 1 {
			snippet := stripHTMLTags(snippetMatches[i][1])
			result.Snippet = strings.TrimSpace(decodeHTMLEntities(snippet))
		}

		results = append(results, result)
		if len(results) >= maxSearchResults {
			break
		}
	}

	// Fallback: try to parse any links from the page if the above didn't work.
	if len(results) == 0 {
		// Look for any external links in table rows.
		rowRe := regexp.MustCompile(`(?s)<tr[^>]*>.*?<a[^>]+href="(https?://[^"]+)"[^>]*>([^<]+)</a>.*?</tr>`)
		rowMatches := rowRe.FindAllStringSubmatch(html, maxSearchResults*2)

		for _, match := range rowMatches {
			if len(match) < 3 {
				continue
			}

			linkURL := match[1]
			title := strings.TrimSpace(match[2])

			// Skip DuckDuckGo internal links.
			if strings.Contains(linkURL, "duckduckgo.com") {
				continue
			}

			results = append(results, searchResult{
				Title: decodeHTMLEntities(title),
				URL:   linkURL,
			})

			if len(results) >= maxSearchResults {
				break
			}
		}
	}

	return results, nil
}

func stripHTMLTags(s string) string {
	tagRe := regexp.MustCompile(`<[^>]+>`)
	return tagRe.ReplaceAllString(s, "")
}

func decodeHTMLEntities(s string) string {
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&apos;", "'")
	return s
}
