package conf

import (
	"os"
	"testing"
)

func TestWebDashBaseURL(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{"default", "", defaultWebDashURL},
		{"custom", "https://custom.com", "https://custom.com"},
		{"custom_trailing", "https://custom.com/", "https://custom.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("ENCORE_WEBDASH_URL", tt.env)
			} else {
				// Ensure env is cleared if running in dirty env
				// On parallel tests this might be an issue but t.Setenv handles it
				os.Unsetenv("ENCORE_WEBDASH_URL")
			}
			if got := WebDashBaseURL(); got != tt.want {
				t.Errorf("WebDashBaseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDocsBaseURL(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{"default", "", defaultDocsURL},
		{"custom", "https://docs.custom.com", "https://docs.custom.com"},
		{"custom_trailing", "https://docs.custom.com/", "https://docs.custom.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("ENCORE_DOCS_URL", tt.env)
			} else {
				os.Unsetenv("ENCORE_DOCS_URL")
			}
			if got := DocsBaseURL(); got != tt.want {
				t.Errorf("DocsBaseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
