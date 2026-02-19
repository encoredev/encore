package editors

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestConvertFilePathToURLScheme(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name         string
		editor       EditorName
		fullPath     string
		startLine    int
		startCol     int
		wantURL      string
		wantAsURL    bool
	}{
		{
			name:      "VSCode without line",
			editor:    VSCode,
			fullPath:  "/path/to/file.go",
			startLine: 0,
			startCol:  0,
			wantURL:   "vscode://file/path/to/file.go",
			wantAsURL: true,
		},
		{
			name:      "VSCode with line",
			editor:    VSCode,
			fullPath:  "/path/to/file.go",
			startLine: 42,
			startCol:  0,
			wantURL:   "vscode://file/path/to/file.go:42",
			wantAsURL: true,
		},
		{
			name:      "VSCode Insiders with line",
			editor:    VSCodeInsiders,
			fullPath:  "/path/to/file.go",
			startLine: 10,
			startCol:  0,
			wantURL:   "vscode://file/path/to/file.go:10",
			wantAsURL: true,
		},
		{
			name:      "Cursor without line",
			editor:    Cursor,
			fullPath:  "/path/to/file.go",
			startLine: 0,
			startCol:  0,
			wantURL:   "cursor://file/path/to/file.go",
			wantAsURL: true,
		},
		{
			name:      "Cursor with line",
			editor:    Cursor,
			fullPath:  "/path/to/file.go",
			startLine: 42,
			startCol:  0,
			wantURL:   "cursor://file/path/to/file.go:42",
			wantAsURL: true,
		},
		{
			name:      "GoLand without line",
			editor:    JetbrainsGoLand,
			fullPath:  "/path/to/file.go",
			startLine: 0,
			startCol:  0,
			wantURL:   "goland://open?file=%2Fpath%2Fto%2Ffile.go",
			wantAsURL: true,
		},
		{
			name:      "GoLand with line and column",
			editor:    JetbrainsGoLand,
			fullPath:  "/path/to/file.go",
			startLine: 42,
			startCol:  10,
			wantURL:   "goland://open?col=10&file=%2Fpath%2Fto%2Ffile.go&line=42",
			wantAsURL: true,
		},
		{
			name:      "WebStorm with line",
			editor:    JetbrainsWebStorm,
			fullPath:  "/path/to/file.ts",
			startLine: 15,
			startCol:  0,
			wantURL:   "webstorm://open?file=%2Fpath%2Fto%2Ffile.ts&line=15",
			wantAsURL: true,
		},
		{
			name:      "TextMate with line and column",
			editor:    TextMate,
			fullPath:  "/path/to/file.txt",
			startLine: 5,
			startCol:  3,
			wantURL:   "txmt://open?col=3&line=5&url=file%3A%2F%2F%2Fpath%2Fto%2Ffile.txt",
			wantAsURL: true,
		},
		{
			name:      "BBEdit with line",
			editor:    BBEdit,
			fullPath:  "/path/to/file.txt",
			startLine: 10,
			startCol:  0,
			wantURL:   "bbedit://open?line=10&url=file%3A%2F%2F%2Fpath%2Fto%2Ffile.txt",
			wantAsURL: true,
		},
		{
			name:      "Unsupported editor",
			editor:    SublimeText,
			fullPath:  "/path/to/file.go",
			startLine: 42,
			startCol:  10,
			wantURL:   "",
			wantAsURL: false,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			gotURL, gotAsURL := convertFilePathToURLScheme(tt.editor, tt.fullPath, tt.startLine, tt.startCol)
			c.Assert(gotURL, qt.Equals, tt.wantURL)
			c.Assert(gotAsURL, qt.Equals, tt.wantAsURL)
		})
	}
}
