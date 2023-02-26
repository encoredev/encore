package directive

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
)

// Directive represents a parsed "encore:" directive.
type Directive struct {
	AST *ast.CommentGroup // the comment group containing the directive

	Name    string   // "foo" in "encore:foo"
	Options []string // options that are enabled ("public" and "raw" in "encore:api public raw path=/foo")
	Fields  []Field  // key-value pairs ({"path": "/foo"} in "encore:api public raw path=/foo")
	Tags    []string // tag names ("tag:foo" in "encore:api public tag:foo")
}

type Field struct {
	Key   string
	Value string
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
		b.WriteString(o)
	}
	for _, f := range d.Fields {
		b.WriteByte(' ')
		b.WriteString(f.Key)
		b.WriteByte('=')
		b.WriteString(f.Value)
	}
	for _, t := range d.Tags {
		b.WriteByte(' ')
		b.WriteString(t)
	}
	return b.String()
}

// HasOption reports whether the directive contains the given option.
func (d Directive) HasOption(name string) bool {
	for _, o := range d.Options {
		if o == name {
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
func Parse(cg *ast.CommentGroup) (dir *Directive, doc string, err error) {
	if cg == nil {
		return nil, cg.Text(), nil
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
			dir, err := parseOne(c.Text[len(prefix):])
			if err != nil {
				return nil, "", err
			}
			dir.AST = cg
			dirs = append(dirs, &dir)
		}
	}
	if len(dirs) == 1 {
		doc := cg.Text() // skips directives for us
		return dirs[0], doc, nil
	} else if len(dirs) > 1 {
		return nil, "", fmt.Errorf("multiple encore directives for same declaration")
	}

	// Legacy syntax
	lines := strings.Split(cg.Text(), "\n")
	var docLines []string

	for _, line := range lines {
		const prefix = "encore:"
		if strings.HasPrefix(line, prefix) {
			dir, err := parseOne(line[len(prefix):])
			if err != nil {
				return nil, "", err
			}
			dir.AST = cg
			dirs = append(dirs, &dir)
			continue
		}
		docLines = append(docLines, line)
	}

	if len(dirs) == 0 {
		return nil, cg.Text(), nil
	} else if len(dirs) > 1 {
		return nil, "", fmt.Errorf("multiple encore directives for same declaration")
	}
	doc = strings.TrimSpace(strings.Join(docLines, "\n"))
	return dirs[0], doc, nil
}

var (
	// nameRe is the regexp for validating option names and field names.
	nameRe = regexp.MustCompile(`^[a-z]+$`)
	// tagRe is the regexp for validating tag values.
	tagRe = regexp.MustCompile(`^[a-z]([-_a-z0-9]*[a-z0-9])?$`)
)

// parseOne parses a single Directive from line.
// It does not set Directive.AST.
func parseOne(line string) (d Directive, err error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return Directive{}, fmt.Errorf("invalid encore directive: missing directive name")
	}

	// Annotate any errors we return.
	defer func() {
		if err != nil {
			err = fmt.Errorf("invalid encore:%s directive: %v", fields[0], err)
		}
	}()

	// seen tracks fields already seen, to detect duplicates.
	seen := make(map[string]bool, len(fields))

	d.Name = fields[0]
	for _, f := range fields[1:] {
		// seenKey is the key to use for detecting duplicates.
		// Default it to the field itself.
		seenKey := f

		if strings.HasPrefix(f, "tag:") {
			if seen[seenKey] {
				return Directive{}, fmt.Errorf("duplicate tag %q", seenKey)
			} else if !tagRe.MatchString(f[len("tag:"):]) {
				return Directive{}, fmt.Errorf("invalid tag %q", f)
			}

			d.Tags = append(d.Tags, f)
		} else if key, value, ok := strings.Cut(f, "="); ok {
			seenKey = key

			if value == "" {
				return Directive{}, fmt.Errorf("field %q has no value", seenKey)
			} else if !nameRe.MatchString(key) {
				return Directive{}, fmt.Errorf("invalid field %q", key)
			} else if seen[seenKey] {
				return Directive{}, fmt.Errorf("duplicate field %q", seenKey)
			}
			d.Fields = append(d.Fields, Field{Key: key, Value: value})
		} else {
			if !nameRe.MatchString(f) {
				return Directive{}, fmt.Errorf("invalid option %q", f)
			} else if seen[seenKey] {
				return Directive{}, fmt.Errorf("duplicate option %q", seenKey)
			}
			d.Options = append(d.Options, f)
		}

		seen[seenKey] = true
	}

	return d, nil
}

type ValidateSpec struct {
	// AllowedFields and AllowedOptions are the allowed fields and options.
	// Expected values must be found in the corresponding list.
	AllowedOptions []string
	AllowedFields  []string

	// ValidateOption, if non-nil, is called for each option in the directive.
	ValidateOption func(string) error

	// ValidateField, if non-nil, is called for each field in the directive.
	ValidateField func(Field) error

	// ValidateTag, if non-nil, is called for each tag in the directive.
	// It is called with the whole tag, including the "tag:" prefix.
	ValidateTag func(string) error
}

// Validate checks that the directive is valid according to spec.
func Validate(d *Directive, spec ValidateSpec) error {
	// Check the options.
	for _, o := range d.Options {
		if !slices.Contains(spec.AllowedOptions, o) {
			return fmt.Errorf("unknown option %q", o)
		}
		if spec.ValidateOption != nil {
			if err := spec.ValidateOption(o); err != nil {
				return fmt.Errorf("invalid option %q: %v", o, err)
			}
		}
	}

	for _, f := range d.Fields {
		if !slices.Contains(spec.AllowedFields, f.Key) {
			return fmt.Errorf("unknown field %q", f.Key)
		}
		if spec.ValidateField != nil {
			if err := spec.ValidateField(f); err != nil {
				return fmt.Errorf("invalid field %q: %v", f.Key, err)
			}
		}
	}
	for _, t := range d.Tags {
		if spec.ValidateTag != nil {
			if err := spec.ValidateTag(t); err != nil {
				return fmt.Errorf("invalid tag %q: %v", t, err)
			}
		}
	}
	return nil
}
