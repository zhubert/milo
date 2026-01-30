// Package lsp provides Language Server Protocol client functionality
// for code navigation across multiple programming languages.
package lsp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// Position represents a position in a text document.
// LSP uses 0-indexed line and character positions.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a span of text in a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location in a specific document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams specifies a position in a document.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// ReferenceParams extends TextDocumentPositionParams for references request.
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// ReferenceContext controls what references are included.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// WorkspaceSymbolParams for workspace/symbol request.
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// SymbolKind represents the kind of a symbol.
type SymbolKind int

// Symbol kinds as defined by LSP.
const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
	SymbolKindObject        SymbolKind = 19
	SymbolKindKey           SymbolKind = 20
	SymbolKindNull          SymbolKind = 21
	SymbolKindEnumMember    SymbolKind = 22
	SymbolKindStruct        SymbolKind = 23
	SymbolKindEvent         SymbolKind = 24
	SymbolKindOperator      SymbolKind = 25
	SymbolKindTypeParameter SymbolKind = 26
)

// String returns a human-readable name for the symbol kind.
func (k SymbolKind) String() string {
	names := map[SymbolKind]string{
		SymbolKindFile:          "file",
		SymbolKindModule:        "module",
		SymbolKindNamespace:     "namespace",
		SymbolKindPackage:       "package",
		SymbolKindClass:         "class",
		SymbolKindMethod:        "method",
		SymbolKindProperty:      "property",
		SymbolKindField:         "field",
		SymbolKindConstructor:   "constructor",
		SymbolKindEnum:          "enum",
		SymbolKindInterface:     "interface",
		SymbolKindFunction:      "function",
		SymbolKindVariable:      "variable",
		SymbolKindConstant:      "constant",
		SymbolKindString:        "string",
		SymbolKindNumber:        "number",
		SymbolKindBoolean:       "boolean",
		SymbolKindArray:         "array",
		SymbolKindObject:        "object",
		SymbolKindKey:           "key",
		SymbolKindNull:          "null",
		SymbolKindEnumMember:    "enum member",
		SymbolKindStruct:        "struct",
		SymbolKindEvent:         "event",
		SymbolKindOperator:      "operator",
		SymbolKindTypeParameter: "type parameter",
	}
	if name, ok := names[k]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", k)
}

// SymbolInformation represents information about a symbol.
type SymbolInformation struct {
	Name          string     `json:"name"`
	Kind          SymbolKind `json:"kind"`
	Location      Location   `json:"location"`
	ContainerName string     `json:"containerName,omitempty"`
}

// Hover represents hover information.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// MarkupContent represents markup content returned in hovers.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// InitializeParams for the initialize request.
type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      string             `json:"rootUri"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities describes client capabilities.
type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

// TextDocumentClientCapabilities describes text document capabilities.
type TextDocumentClientCapabilities struct {
	Hover HoverClientCapabilities `json:"hover,omitempty"`
}

// HoverClientCapabilities describes hover capabilities.
type HoverClientCapabilities struct {
	ContentFormat []string `json:"contentFormat,omitempty"`
}

// InitializeResult is the result of the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// ServerCapabilities describes server capabilities.
type ServerCapabilities struct {
	TextDocumentSync           any  `json:"textDocumentSync,omitempty"`
	HoverProvider              bool `json:"hoverProvider,omitempty"`
	DefinitionProvider         bool `json:"definitionProvider,omitempty"`
	ReferencesProvider         bool `json:"referencesProvider,omitempty"`
	WorkspaceSymbolProvider    bool `json:"workspaceSymbolProvider,omitempty"`
	DocumentSymbolProvider     bool `json:"documentSymbolProvider,omitempty"`
	CompletionProvider         any  `json:"completionProvider,omitempty"`
	SignatureHelpProvider      any  `json:"signatureHelpProvider,omitempty"`
	CodeActionProvider         any  `json:"codeActionProvider,omitempty"`
	DocumentFormattingProvider bool `json:"documentFormattingProvider,omitempty"`
	RenameProvider             any  `json:"renameProvider,omitempty"`
}

// FileToURI converts a file path to a file:// URI.
func FileToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	// On Windows, paths need special handling
	abs = filepath.ToSlash(abs)
	if !strings.HasPrefix(abs, "/") {
		abs = "/" + abs
	}
	return "file://" + abs
}

// URIToFile converts a file:// URI to a file path.
func URIToFile(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("parsing URI: %w", err)
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("expected file:// URI, got %s", parsed.Scheme)
	}
	path := parsed.Path
	// Handle Windows paths (e.g., /C:/path)
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path), nil
}

// ToLSPPosition converts 1-indexed line and column to 0-indexed LSP position.
func ToLSPPosition(line, column int) Position {
	return Position{
		Line:      line - 1,
		Character: column - 1,
	}
}

// FromLSPPosition converts 0-indexed LSP position to 1-indexed line and column.
func FromLSPPosition(pos Position) (line, column int) {
	return pos.Line + 1, pos.Character + 1
}
