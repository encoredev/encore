package userconfig

import (
	"fmt"
	"strings"
)

//go:generate go run ./gendocs

func CLIDocs() string {
	var buf strings.Builder
	for _, key := range configKeys {
		desc := descs[key]
		doc := desc.Doc
		fmt.Fprintf(&buf, "%s (%s)\n", key, desc.Type.Kind.String())

		if doc != "" {
			rem := doc
			for rem != "" {
				var line string
				if idx := strings.IndexByte(rem, '\n'); idx != -1 {
					line = rem[:idx]
					rem = rem[idx+1:]
				} else {
					line = rem
					rem = ""
				}
				buf.WriteString("  ")
				buf.WriteString(line)
				buf.WriteByte('\n')
			}
		} else {
			buf.WriteString("  No documentation available.\n")
		}

		buf.WriteByte('\n')

		didWriteMore := false
		if desc.Type.Default != nil {
			fmt.Fprintf(&buf, "  Default: %v\n", RenderValue(*desc.Type.Default))
			didWriteMore = true
		}
		if len(desc.Type.Oneof) > 0 {
			fmt.Fprintf(&buf, "  Must be one of: %v\n", RenderOneof(desc.Type.Oneof))
			didWriteMore = true
		}

		// Add an extra newline if we wrote validation details.
		if didWriteMore {
			buf.WriteByte('\n')
		}
	}

	return buf.String()
}

// bt renders a backtick-enclosed string.
func bt(val string) string {
	return fmt.Sprintf("`%s`", val)
}

var markdownHeader = `
The Encore CLI has a number of configuration options to customize its behavior.

Configuration options can be set both for individual Encore applications, as well as
globally for the local user.

Configuration options can be set using ` + bt("encore config <key> <value>") + `,
and options can similarly be read using ` + bt("encore config <key>") + `.

When running ` + bt("encore config") + ` within an Encore application, it automatically
sets and gets configuration for that application.

To set or get global configuration, use the ` + bt("--global") + ` flag.

## Configuration files

The configuration is stored in one ore more TOML files on the filesystem.

The configuration is read from the following files, in order:

### Global configuration
* ` + bt("$XDG_CONFIG_HOME/encore/config") + `
* ` + bt("$HOME/.config/encore/config") + `
* ` + bt("$HOME/.encoreconfig") + `

### Application-specific configuration
* ` + bt("$APP_ROOT/.encore/config") + `

Where ` + bt("$APP_ROOT") + ` is the directory containing the ` + bt("encore.app") + ` file.

The files are read and merged, in the order defined above, with latter files taking precedence over earlier files.

## Configuration options

`

func MarkdownDocs() string {
	var buf strings.Builder

	buf.WriteString(markdownHeader)

	for _, key := range configKeys {
		desc := descs[key]
		doc := desc.Doc

		fmt.Fprintf(&buf, "#### %s\n", key)
		fmt.Fprintf(&buf, "Type: %s<br/>\n", desc.Type.Kind.String())
		if desc.Type.Default != nil {
			fmt.Fprintf(&buf, "Default: %v<br/>\n", RenderValue(*desc.Type.Default))
		}
		if len(desc.Type.Oneof) > 0 {
			fmt.Fprintf(&buf, "Must be one of: %v\n", RenderOneof(desc.Type.Oneof))
		}
		buf.WriteByte('\n')

		if doc != "" {
			buf.WriteString(doc)
		} else {
			buf.WriteString("No documentation available.\n")
		}

		buf.WriteByte('\n')
	}

	return buf.String()
}
