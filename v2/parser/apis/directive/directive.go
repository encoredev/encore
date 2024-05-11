package directive

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"slices"
	"strings"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/idents"
	"encr.dev/v2/internals/perr"
)

// Directive represents a parsed "encore:" directive.
type Directive struct {
	AST *ast.CommentGroup // the comment group containing the directive

	Name    string  // "foo" in "encore:foo"
	Options []Field // options that are enabled ("public" and "raw" in "encore:api public raw path=/foo")
	Fields  []Field // key-value pairs ({"path": "/foo"} in "encore:api public raw path=/foo")
	Tags    []Field // tag names ("tag:foo" in "encore:api public tag:foo")

	start   token.Pos // start position of the directive
	nameEnd token.Pos
	end     token.Pos // end position of the directive
}

var _ ast.Node = (*Directive)(nil)

func (d Directive) Pos() token.Pos {
	return d.start
}

func (d Directive) End() token.Pos {
	return d.end
}

type Field struct {
	Key   string
	Value string

	start token.Pos // position of the key
	end   token.Pos // position of the end value
}

// Equal reports whether two fields are equal.
// It's implemented for testing purposes.
func (f Field) Equal(other Field) bool {
	return f.Key == other.Key && f.Value == other.Value
}

var _ ast.Node = (*Field)(nil)

func (f Field) Pos() token.Pos {
	return f.start
}

func (f Field) End() token.Pos {
	return f.end
}

// List returns the field value as a list, split by commas.
func (f Field) List() []string {
	return strings.Split(f.Value, ",")
}

// String returns the string representation of d.
func (d Directive) String() string {
	var b strings.Builder
	b.WriteString("encore:")
	b.WriteString(d.Name)
	for _, o := range d.Options {
		b.WriteByte(' ')
		b.WriteString(o.Value)
	}
	for _, f := range d.Fields {
		b.WriteByte(' ')
		b.WriteString(f.Key)
		b.WriteByte('=')
		b.WriteString(f.Value)
	}
	for _, t := range d.Tags {
		b.WriteByte(' ')
		b.WriteString(t.Value)
	}
	return b.String()
}

// HasOption reports whether the directive contains the given option.
func (d Directive) HasOption(name string) bool {
	for _, o := range d.Options {
		if o.Value == name {
			return true
		}
	}
	return false
}

// Get returns the value of the given field, if any.
// If the field doesn't exist it reports "".
func (d Directive) Get(name string) string {
	for _, f := range d.Fields {
		if f.Key == name {
			return f.Value
		}
	}
	return ""
}

// GetList returns the value of the given field, split by commas.
// If the field doesn't exist it reports nil.
func (d Directive) GetList(name string) []string {
	for _, f := range d.Fields {
		if f.Key == name {
			return f.List()
		}
	}
	return nil
}

// Parse parses the encore:foo directives in cg.
// It returns the parsed directives, if any, and the
// remaining doc text after stripping the directive lines.
func Parse(errs *perr.List, cg *ast.CommentGroup) (dir *Directive, doc string, ok bool) {
	if cg == nil {
		return nil, "", true
	}

	// Go has standardized on directives in the form "//[a-z0-9]+:[a-z0-9+]".
	// Encore has allowed a space between // and the Directive,
	// but we would like to migrate to the standard syntax.
	//
	// First try the standard syntax and fall back to the legacy syntax
	// if we don't find any directives.

	// Standard syntax
	var dirs []*Directive
	for _, c := range cg.List {
		const prefix = "//encore:"
		if strings.HasPrefix(c.Text, prefix) {
			dir, ok := parseOne(errs, c.Pos()+2, c.Text[len(prefix):])
			if !ok {
				return nil, "", false
			}
			dir.AST = cg
			dirs = append(dirs, &dir)
		}
	}
	if len(dirs) == 1 {
		doc := cg.Text() // skips directives for us
		return dirs[0], doc, true
	} else if len(dirs) > 1 {
		err := errMultipleDirectives
		for _, dir := range dirs {
			err = err.AtGoNode(dir)
		}
		errs.Add(err)
		return nil, "", false
	}

	// Legacy syntax
	lines := strings.Split(cg.Text(), "\n")
	var docLines []string

	for _, line := range lines {
		const prefix = "encore:"
		if strings.HasPrefix(line, prefix) {
			pos := cg.Pos()

			// Find the position of the directive.
			for _, c := range cg.List {
				idx := bytes.Index([]byte(c.Text), []byte(line))
				if idx >= 0 {
					pos = c.Pos() + token.Pos(idx)
					break
				}
			}

			dir, ok := parseOne(errs, pos, line[len(prefix):])
			if !ok {
				return nil, "", false
			}
			dir.AST = cg
			dirs = append(dirs, &dir)
			continue
		}
		docLines = append(docLines, line)
	}

	if len(dirs) == 0 {
		return nil, cg.Text(), true
	} else if len(dirs) > 1 {
		err := errMultipleDirectives
		for _, dir := range dirs {
			err = err.AtGoNode(dir)
		}
		errs.Add(err)
		return nil, "", false
	}
	doc = strings.TrimSpace(strings.Join(docLines, "\n"))
	return dirs[0], doc, true
}

var (
	// nameRe is the regexp for validating option names and field names.
	nameRe = regexp.MustCompile(`^[a-z]+$`)
	// tagRe is the regexp for validating tag values.
	tagRe = regexp.MustCompile(`^[a-z]([-_a-z0-9]*[a-z0-9])?$`)
)

// parseOne parses a single Directive from line.
// It does not set Directive.AST.
func parseOne(errs *perr.List, pos token.Pos, line string) (d Directive, ok bool) {
	fields := fields(pos+7, line) // +7 for "encore:"
	if len(fields) == 0 {
		errs.Add(errMissingDirectiveName.AtGoPos(pos, pos+7+token.Pos(len([]byte(line)))))
		return Directive{}, false
	}

	// seen tracks fields already seen, to detect duplicates.
	seen := make(map[string]Field, len(fields))

	d.Name = fields[0].Value
	d.start = pos
	d.nameEnd = pos + 7 + token.Pos(len([]byte(d.Name)))
	d.end = pos + +7 + token.Pos(len([]byte(line)))

	for _, f := range fields[1:] {
		// seenKey is the key to use for detecting duplicates.
		// Default it to the field itself.
		seenKey := f.Value

		if strings.HasPrefix(f.Value, "tag:") {
			tag := f.Value[len("tag:"):]

			if other, found := seen[seenKey]; found {
				errs.Add(errDuplicateTag(seenKey).AtGoNode(other).AtGoNode(f))
				return Directive{}, false
			} else if !tagRe.MatchString(tag) {
				errs.Add(errInvalidTag(tag).
					AtGoPos(f.start+4, f.end, errors.AsError(
						fmt.Sprintf("try %q?", idents.GenerateSuggestion(tag, idents.KebabCase)),
					)))
				return Directive{}, false
			}

			d.Tags = append(d.Tags, f)
		} else if key, value, ok := strings.Cut(f.Value, "="); ok {
			seenKey = key
			f.Key = key
			f.Value = value

			if value == "" {
				errs.Add(errFieldHasNoValue.AtGoNode(f))
				return Directive{}, false
			} else if !nameRe.MatchString(key) {
				errs.Add(errInvalidFieldName(key).AtGoPos(f.start, f.start+token.Pos(len([]byte(key)))))
				return Directive{}, false
			} else if other, found := seen[seenKey]; found {
				errs.Add(errDuplicateField(seenKey).AtGoNode(f).AtGoNode(other))
				return Directive{}, false
			}
			d.Fields = append(d.Fields, f)
		} else {
			if !nameRe.MatchString(f.Value) {
				errs.Add(errInvalidOptionName(f.Value).AtGoNode(f))
				return Directive{}, false
			} else if other, found := seen[seenKey]; found {
				errs.Add(errDuplicateOption(seenKey).AtGoNode(f).AtGoNode(other))
				return Directive{}, false
			}
			d.Options = append(d.Options, f)
		}

		seen[seenKey] = f
	}

	return d, true
}

type ValidateSpec struct {
	// AllowedFields and AllowedOptions are the allowed fields and options.
	// Expected values must be found in the corresponding list.
	AllowedOptions []string
	AllowedFields  []string

	// ValidateOption, if non-nil, is called for each option in the directive.
	ValidateOption func(*perr.List, Field) (ok bool)

	// ValidateField, if non-nil, is called for each field in the directive.
	ValidateField func(*perr.List, Field) (ok bool)

	// ValidateTag, if non-nil, is called for each tag in the directive.
	// It is called with the whole tag, including the "tag:" prefix.
	ValidateTag func(*perr.List, Field) (ok bool)
}

// Validate checks that the directive is valid according to spec.
func Validate(errs *perr.List, d *Directive, spec ValidateSpec) (ok bool) {
	// Check the options.
	for _, o := range d.Options {
		if !slices.Contains(spec.AllowedOptions, o.Value) {
			errs.Add(errUnknownOption(o.Value, strings.Join(spec.AllowedOptions, ", ")).AtGoNode(o))
			return false
		}
		if spec.ValidateOption != nil {
			if !spec.ValidateOption(errs, o) {
				return false
			}
		}
	}

	for _, f := range d.Fields {
		if !slices.Contains(spec.AllowedFields, f.Key) {
			errs.Add(errUnknownField(f.Key, strings.Join(spec.AllowedFields, ", ")).AtGoNode(f))
			return false
		}
		if spec.ValidateField != nil {
			if !spec.ValidateField(errs, f) {
				return false
			}
		}
	}
	for _, t := range d.Tags {
		if spec.ValidateTag != nil {
			if !spec.ValidateTag(errs, t) {
				return false
			}
		}
	}
	return true
}
