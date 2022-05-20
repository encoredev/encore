package codegen

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
		{"getVersion2", []string{"get", "version", "2"}},
		{"GetAPIDocs", []string{"get", "API", "docs"}},
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
		{"getVersion2", "getVersion2", "GetVersion2", "get_version_2", "GET_VERSION_2", "get-version-2"},
		{"GetAPIDocs", "getAPIDocs", "GetAPIDocs", "get_api_docs", "GET_API_DOCS", "get-api-docs"},
	}

	for _, tt := range tests {
		tt := tt
		c.Run(tt.input, func(c *qt.C) {
			c.Parallel()

			c.Assert(convertIdentifierTo(tt.input, CamelCase), qt.Equals, tt.camelCase)
			c.Assert(convertIdentifierTo(tt.input, PascalCase), qt.Equals, tt.pascalCase)
			c.Assert(convertIdentifierTo(tt.input, SnakeCase), qt.Equals, tt.snakeCase)
			c.Assert(convertIdentifierTo(tt.input, ScreamingSnakeCase), qt.Equals, tt.screamingSnakeCase)
			c.Assert(convertIdentifierTo(tt.input, KebabCase), qt.Equals, tt.kebabCase)
		})
	}
}
