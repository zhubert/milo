package lsp

import (
	"runtime"
	"testing"
)

func TestFileToURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "absolute Unix path",
			path: "/path/to/file.go",
			want: "file:///path/to/file.go",
		},
		{
			name: "path with spaces",
			path: "/path/to/my file.go",
			want: "file:///path/to/my file.go",
		},
	}

	// Skip Windows-specific tests on non-Windows
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name string
			path string
			want string
		}{
			name: "Windows path",
			path: "C:\\Users\\test\\file.go",
			want: "file:///C:/Users/test/file.go",
		})
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FileToURI(tc.path)
			if got != tc.want {
				t.Errorf("FileToURI(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestURIToFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name: "file URI",
			uri:  "file:///path/to/file.go",
			want: "/path/to/file.go",
		},
		{
			name:    "non-file URI",
			uri:     "http://example.com/file.go",
			wantErr: true,
		},
		{
			name:    "invalid URI",
			uri:     ":::invalid",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := URIToFile(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Errorf("URIToFile(%q) expected error, got nil", tc.uri)
				}
				return
			}
			if err != nil {
				t.Errorf("URIToFile(%q) unexpected error: %v", tc.uri, err)
				return
			}
			if got != tc.want {
				t.Errorf("URIToFile(%q) = %q, want %q", tc.uri, got, tc.want)
			}
		})
	}
}

func TestToLSPPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line, col int
		wantLine  int
		wantChar  int
	}{
		{1, 1, 0, 0},
		{10, 5, 9, 4},
		{100, 50, 99, 49},
	}

	for _, tc := range tests {
		pos := ToLSPPosition(tc.line, tc.col)
		if pos.Line != tc.wantLine || pos.Character != tc.wantChar {
			t.Errorf("ToLSPPosition(%d, %d) = {%d, %d}, want {%d, %d}",
				tc.line, tc.col, pos.Line, pos.Character, tc.wantLine, tc.wantChar)
		}
	}
}

func TestFromLSPPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pos      Position
		wantLine int
		wantCol  int
	}{
		{Position{0, 0}, 1, 1},
		{Position{9, 4}, 10, 5},
		{Position{99, 49}, 100, 50},
	}

	for _, tc := range tests {
		line, col := FromLSPPosition(tc.pos)
		if line != tc.wantLine || col != tc.wantCol {
			t.Errorf("FromLSPPosition({%d, %d}) = (%d, %d), want (%d, %d)",
				tc.pos.Line, tc.pos.Character, line, col, tc.wantLine, tc.wantCol)
		}
	}
}

func TestSymbolKindString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind SymbolKind
		want string
	}{
		{SymbolKindFunction, "function"},
		{SymbolKindClass, "class"},
		{SymbolKindMethod, "method"},
		{SymbolKindVariable, "variable"},
		{SymbolKindStruct, "struct"},
		{SymbolKind(999), "unknown(999)"},
	}

	for _, tc := range tests {
		got := tc.kind.String()
		if got != tc.want {
			t.Errorf("SymbolKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestPositionConversionRoundTrip(t *testing.T) {
	t.Parallel()

	// Test that converting to LSP and back gives original values
	for line := 1; line <= 100; line++ {
		for col := 1; col <= 50; col++ {
			pos := ToLSPPosition(line, col)
			gotLine, gotCol := FromLSPPosition(pos)
			if gotLine != line || gotCol != col {
				t.Errorf("round trip failed: (%d, %d) -> {%d, %d} -> (%d, %d)",
					line, col, pos.Line, pos.Character, gotLine, gotCol)
			}
		}
	}
}
