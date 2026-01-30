package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// DiffTool compares two files or a file against provided content.
// It implements ParallelSafeTool since it only reads data.
type DiffTool struct{}

// IsParallelSafe returns true since diff operations don't modify state.
func (t *DiffTool) IsParallelSafe() bool { return true }

type diffInput struct {
	FilePath1 string `json:"file_path_1"`
	FilePath2 string `json:"file_path_2"`
	Content   string `json:"content"`
	Context   int    `json:"context"`
}

func (t *DiffTool) Name() string { return "diff" }

func (t *DiffTool) Description() string {
	return "Compare two files or compare a file against provided content. " +
		"Returns a unified diff showing the differences. " +
		"Use file_path_1 and file_path_2 to compare two files, or file_path_1 and content to compare a file against provided text."
}

func (t *DiffTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"file_path_1": map[string]any{
				"type":        "string",
				"description": "The absolute path to the first file (original)",
			},
			"file_path_2": map[string]any{
				"type":        "string",
				"description": "The absolute path to the second file (modified). Either file_path_2 or content must be provided.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to compare against file_path_1. Either file_path_2 or content must be provided.",
			},
			"context": map[string]any{
				"type":        "integer",
				"description": "Number of context lines to show around changes (default: 3)",
			},
		},
		Required: []string{"file_path_1"},
	}
}

func (t *DiffTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in diffInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing diff input: %w", err)
	}

	if !filepath.IsAbs(in.FilePath1) {
		return Result{Output: "file_path_1 must be an absolute path", IsError: true}, nil
	}

	if in.FilePath2 == "" && in.Content == "" {
		return Result{Output: "either file_path_2 or content must be provided", IsError: true}, nil
	}

	if in.FilePath2 != "" && in.Content != "" {
		return Result{Output: "provide only one of file_path_2 or content, not both", IsError: true}, nil
	}

	if in.FilePath2 != "" && !filepath.IsAbs(in.FilePath2) {
		return Result{Output: "file_path_2 must be an absolute path", IsError: true}, nil
	}

	// Read the first file.
	data1, err := os.ReadFile(in.FilePath1)
	if err != nil {
		return Result{Output: fmt.Sprintf("error reading file_path_1: %s", err), IsError: true}, nil
	}

	// Get the second content.
	var data2 []byte
	var label2 string
	if in.FilePath2 != "" {
		data2, err = os.ReadFile(in.FilePath2)
		if err != nil {
			return Result{Output: fmt.Sprintf("error reading file_path_2: %s", err), IsError: true}, nil
		}
		label2 = in.FilePath2
	} else {
		data2 = []byte(in.Content)
		label2 = "(provided content)"
	}

	contextLines := 3
	if in.Context > 0 {
		contextLines = in.Context
	}

	diff := unifiedDiff(in.FilePath1, label2, string(data1), string(data2), contextLines)

	if diff == "" {
		return Result{Output: "Files are identical"}, nil
	}

	return Result{Output: diff}, nil
}

// unifiedDiff generates a unified diff between two strings.
func unifiedDiff(label1, label2, text1, text2 string, contextLines int) string {
	lines1 := splitLines(text1)
	lines2 := splitLines(text2)

	// Compute LCS-based diff.
	edits := computeDiff(lines1, lines2)

	if len(edits) == 0 {
		return ""
	}

	// Group edits into hunks.
	hunks := groupHunks(edits, len(lines1), len(lines2), contextLines)

	if len(hunks) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n", label1)
	fmt.Fprintf(&b, "+++ %s\n", label2)

	for _, hunk := range hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", hunk.oldStart+1, hunk.oldCount, hunk.newStart+1, hunk.newCount)
		for _, line := range hunk.lines {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return b.String()
}

type edit struct {
	kind    int // -1 = delete, 0 = keep, 1 = insert
	oldLine int
	newLine int
	text    string
}

type hunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []string
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// Remove trailing empty line from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// computeDiff uses a simple LCS-based algorithm to find differences.
func computeDiff(lines1, lines2 []string) []edit {
	m, n := len(lines1), len(lines2)

	// Build LCS table.
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if lines1[i] == lines2[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				if lcs[i+1][j] > lcs[i][j+1] {
					lcs[i][j] = lcs[i+1][j]
				} else {
					lcs[i][j] = lcs[i][j+1]
				}
			}
		}
	}

	// Backtrack to produce edits.
	var edits []edit
	i, j := 0, 0
	for i < m || j < n {
		if i < m && j < n && lines1[i] == lines2[j] {
			edits = append(edits, edit{kind: 0, oldLine: i, newLine: j, text: lines1[i]})
			i++
			j++
		} else if j < n && (i >= m || lcs[i][j+1] >= lcs[i+1][j]) {
			edits = append(edits, edit{kind: 1, oldLine: i, newLine: j, text: lines2[j]})
			j++
		} else {
			edits = append(edits, edit{kind: -1, oldLine: i, newLine: j, text: lines1[i]})
			i++
		}
	}

	return edits
}

// groupHunks groups edits into hunks with context.
func groupHunks(edits []edit, oldLen, newLen, contextLines int) []hunk {
	// Find ranges of changes.
	var changeRanges [][2]int // start, end indices in edits
	inChange := false
	start := 0

	for i, e := range edits {
		if e.kind != 0 {
			if !inChange {
				inChange = true
				start = i
			}
		} else {
			if inChange {
				changeRanges = append(changeRanges, [2]int{start, i})
				inChange = false
			}
		}
	}
	if inChange {
		changeRanges = append(changeRanges, [2]int{start, len(edits)})
	}

	if len(changeRanges) == 0 {
		return nil
	}

	// Expand ranges with context and merge overlapping.
	var mergedRanges [][2]int
	for _, r := range changeRanges {
		expandedStart := r[0] - contextLines
		if expandedStart < 0 {
			expandedStart = 0
		}
		expandedEnd := r[1] + contextLines
		if expandedEnd > len(edits) {
			expandedEnd = len(edits)
		}

		if len(mergedRanges) > 0 && expandedStart <= mergedRanges[len(mergedRanges)-1][1] {
			// Merge with previous.
			mergedRanges[len(mergedRanges)-1][1] = expandedEnd
		} else {
			mergedRanges = append(mergedRanges, [2]int{expandedStart, expandedEnd})
		}
	}

	// Build hunks.
	var hunks []hunk
	for _, r := range mergedRanges {
		h := hunk{}
		rangeEdits := edits[r[0]:r[1]]

		if len(rangeEdits) == 0 {
			continue
		}

		// Calculate old/new line ranges.
		firstEdit := rangeEdits[0]
		h.oldStart = firstEdit.oldLine
		h.newStart = firstEdit.newLine

		for _, e := range rangeEdits {
			switch e.kind {
			case -1:
				h.lines = append(h.lines, "-"+e.text)
				h.oldCount++
			case 1:
				h.lines = append(h.lines, "+"+e.text)
				h.newCount++
			case 0:
				h.lines = append(h.lines, " "+e.text)
				h.oldCount++
				h.newCount++
			}
		}

		hunks = append(hunks, h)
	}

	return hunks
}
