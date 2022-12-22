package idents

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func Test_parseIdentifier(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	tests := []struct {
		input  string
		expect []string
	}{
		{"hello", []string{"hello"}},
		{"Hello", []string{"hello"}},
		{"HelloWorld", []string{"hello", "world"}},
		{"hello_world", []string{"hello", "world"}},
		{"Hello_World", []string{"hello", "world"}},
		{"_Hello___World__", []string{"hello", "world"}},
		{"RenderMarkdown", []string{"render", "markdown"}},
		{"RenderHTML", []string{"render", "HTML"}},
		{"getVersion2", []string{"get", "version2"}},
		{"GetAPIDocs", []string{"get", "API", "docs"}},
		{"EncoreResource-123abc", []string{"encore", "resource", "123abc"}},
		{"EncoreResource-abs-123", []string{"encore", "resource", "abs", "123"}},
		{"This is a full sentence... with \"random! bits-and_pieces123 blah", []string{"this", "is", "a", "full", "sentence", "with", "random", "bits", "and", "pieces123", "blah"}},
	}
	for _, tt := range tests {
		tt := tt
		c.Run(tt.input, func(c *qt.C) {
			c.Parallel()

			gotParts := parseIdentifier(tt.input)
			c.Assert(gotParts, qt.DeepEquals, tt.expect)
		})
	}
}

func Test_convertIdentifierTo(t *testing.T) {
	c := qt.New(t)
	c.Parallel()
	type args struct {
		input              string
		camelCase          string
		pascalCase         string
		snakeCase          string
		screamingSnakeCase string
		kebabCase          string
	}
	tests := []args{
		{"Hello", "hello", "Hello", "hello", "HELLO", "hello"},
		{"HelloWorld", "helloWorld", "HelloWorld", "hello_world", "HELLO_WORLD", "hello-world"},
		{"getVersion2", "getVersion2", "GetVersion2", "get_version2", "GET_VERSION2", "get-version2"},
		{"GetAPIDocs", "getAPIDocs", "GetAPIDocs", "get_api_docs", "GET_API_DOCS", "get-api-docs"},
		{"EncoreResource-123abc", "encoreResource123abc", "EncoreResource123abc", "encore_resource_123abc", "ENCORE_RESOURCE_123ABC", "encore-resource-123abc"},
	}

	for _, tt := range tests {
		tt := tt
		c.Run(tt.input, func(c *qt.C) {
			c.Parallel()

			c.Assert(Convert(tt.input, CamelCase), qt.Equals, tt.camelCase)
			c.Assert(Convert(tt.input, PascalCase), qt.Equals, tt.pascalCase)
			c.Assert(Convert(tt.input, SnakeCase), qt.Equals, tt.snakeCase)
			c.Assert(Convert(tt.input, ScreamingSnakeCase), qt.Equals, tt.screamingSnakeCase)
			c.Assert(Convert(tt.input, KebabCase), qt.Equals, tt.kebabCase)
		})
	}
}
