package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"encr.dev/parser/est"
	"encr.dev/parser/paths"
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
		}
	FieldLoop:
		for _, field := range fields[1:] {
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
				if strings.Contains(field, "=") {
					parts := strings.SplitN(field, "=", 2)
					switch parts[0] {
					case "path":
						var err error
						rpc.Path, err = paths.Parse(pos, parts[1])
						if err != nil {
							return nil, fmt.Errorf("invalid API path: %v", err)
						}
						continue FieldLoop
					}
				}
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
	switch {
	case d.Access == est.Private && d.Raw:
		// We don't support private raw APIs for now
		return errors.New("private APIs cannot be declared raw")
	case d.Path != nil && !d.Raw:
		// We only support custom paths for raw APIs for now.
		return errors.New("API path can only be specified for raw APIs")
	default:
		return nil
	}
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
	Path     *paths.Path // nil if not specified
}

// An authHandlerDirective is the parsed representation of the encore:authhandler directive.
type authHandlerDirective struct {
	TokenPos token.Pos
}

func (d *rpcDirective) Pos() token.Pos         { return d.TokenPos }
func (d *authHandlerDirective) Pos() token.Pos { return d.TokenPos }
func (*rpcDirective) directive()               {}
func (*authHandlerDirective) directive()       {}
