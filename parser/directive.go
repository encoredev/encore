package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"net/url"
	"strings"

	"encr.dev/parser/est"
)

const directiveParamPath = "path"

// parseDirectives parses the encore:foo directives in cg.
// It returns the parsed directive, if any, and the
// remaining doc text after stripping the directive lines.
//
// If no directive was found, it reports nil, "".
func (p *parser) parseDirectives(cg *ast.CommentGroup) (d directive, doc string) {
	if cg == nil {
		return nil, ""
	}

	// Go has standardized on directives in the form "//[a-z0-9]+:[a-z0-9+]".
	// Encore has allowed a space between // and the directive,
	// but we would like to migrate to the standard syntax.
	//
	// First try the standard syntax and fall back to the legacy syntax
	// if we don't find any directives.

	var dir directive

	// Standard syntax
	for _, c := range cg.List {
		const prefix = "//encore:"
		if strings.HasPrefix(c.Text, prefix) {
			if dir != nil {
				p.err(c.Pos(), "cannot have multiple encore annotations")
				continue
			}
			var err error
			dir, err = parseDirective(c.Pos(), c.Text[len(prefix):])
			if err != nil {
				p.err(c.Pos(), err.Error())
			}
			err = validateDirective(dir)
			if err != nil {
				p.err(cg.Pos(), err.Error())
			}
		}
	}
	if dir != nil {
		doc := cg.Text() // skips directives for us
		return dir, doc
	}

	// Legacy syntax
	lines := strings.Split(cg.Text(), "\n")
	var docLines []string

LineLoop:
	for _, line := range lines {
		const prefix = "encore:"
		if strings.HasPrefix(line, prefix) {
			if dir != nil {
				p.err(cg.Pos(), "cannot have multiple encore annotations")
				continue
			}
			var err error
			dir, err = parseDirective(cg.Pos(), line[len(prefix):])
			if err != nil {
				p.err(cg.Pos(), err.Error())
			}
			err = validateDirective(dir)
			if err != nil {
				p.err(cg.Pos(), err.Error())
			}

			continue LineLoop
		}
		docLines = append(docLines, line)
	}
	if dir == nil {
		return nil, ""
	}
	doc = strings.TrimSpace(strings.Join(docLines, "\n"))
	return dir, doc
}

// parseDirective parses a single directive from line.
func parseDirective(pos token.Pos, line string) (directive, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, fmt.Errorf("invalid encore directive: %q", line)
	}
	switch fields[0] {
	default:
		return nil, fmt.Errorf("invalid encore directive: %q", fields[0])

	case "api":
		rpc := &rpcDirective{
			TokenPos: pos,
			Access:   est.Private,
			Params:   map[string]string{},
		}
		for _, field := range fields[1:] {
			if isFieldParam(field) {
				parts := strings.SplitN(field, "=", 2)
				_, exists := rpc.Params[parts[0]]
				if exists {
					return nil, errors.New("cannot declare duplicate parameter fields")
				}
				rpc.Params[parts[0]] = parts[1]
				continue
			}

			switch field {
			case "public":
				rpc.Access = est.Public
			case "private":
				rpc.Access = est.Private
			case "auth":
				rpc.Access = est.Auth
			case "raw":
				rpc.Raw = true
			default:
				return nil, fmt.Errorf("unrecognized encore:api directive field: %q", field)
			}
		}

		return rpc, nil

	case "authhandler":
		if len(fields) > 1 {
			return nil, fmt.Errorf("unrecognized encore:authhandler directive field: %q", fields[1])
		}
		return &authHandlerDirective{TokenPos: pos}, nil
	}
}

func isFieldParam(field string) bool {
	return strings.Contains(field, "=")
}

func validateDirective(d directive) error {
	switch td := d.(type) {
	case *rpcDirective:
		return validateRPCDirective(td)
	case *authHandlerDirective:
		return nil
	default:
		return errors.New("unexpected directive type")
	}
}

// validateRPCDirective ensures that the parsed RPC directive is valid.

func validateRPCDirective(d *rpcDirective) error {
	// We don't support private raw APIs for now
	if d.Raw && d.Access == est.Private {
		return errors.New("private APIs cannot be declared raw")
	}

	path, exists := d.Params[directiveParamPath]
	if exists && !d.Raw {
		if !d.Raw {
			return errors.New("path param can only currently be used with raw endpoints")
		}
		pathErr := validateRPCPath(path)
		if pathErr != nil {
			return pathErr
		}
	}
	return nil
}

// Note that this does not include a comprehensive check for path validity but should cover
// most common cases.
func validateRPCPath(path string) error {
	if path == "" {
		return errors.New("path must be non-empty if specified")
	}
	// Use Go's parser to validate that the path is valid.
	_, err := url.Parse(path)
	if err != nil {
		return err
	}

	// Additionally check that there is at most one wildcard
	if strings.Count(path, "*") > 1 {
		return errors.New("path must only contain a single wildcard operator")
	}
	parts := strings.Split(path, "/")
	for _, p := range parts {
		colonCount := strings.Count(p, ":")
		if colonCount > 1 {
			return errors.New("path segments can only contain a single ':' identifier")
		}
		// Ensure the colon is at the start of the string
		if colonCount == 1 && !strings.HasPrefix(p, ":") {
			return errors.New("identifiers ':' must be at the start of a path segment")
		}
	}
	return nil
}

// The directive interface is a marker interface for the directive types we support.
type directive interface {
	Pos() token.Pos
	directive()
}

// An rpcDirective is the parsed representation of the encore:api directive.
type rpcDirective struct {
	TokenPos token.Pos
	Access   est.AccessType
	Raw      bool
	Params   map[string]string
}

// An authHandlerDirective is the parsed representation of the encore:authhandler directive.
type authHandlerDirective struct {
	TokenPos token.Pos
}

func (d *rpcDirective) Pos() token.Pos         { return d.TokenPos }
func (d *authHandlerDirective) Pos() token.Pos { return d.TokenPos }
func (*rpcDirective) directive()               {}
func (*authHandlerDirective) directive()       {}
