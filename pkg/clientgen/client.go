// Package clientgen generates code for use with Encore apps.
package clientgen

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"encr.dev/pkg/clientgen/clientgentypes"
	"encr.dev/pkg/clientgen/openapi"
	"encr.dev/pkg/errinsrc/srcerrors"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Lang represents a programming language or dialect that we support generating code for.
type Lang string

// These constants represent supported languages.
const (
	LangUnknown    Lang = ""
	LangTypeScript Lang = "typescript"
	LangJavascript Lang = "javascript"
	LangGo         Lang = "go"
	LangOpenAPI    Lang = "openapi"
)

type generator interface {
	Generate(p clientgentypes.GenerateParams) error
	Version() int // The version of the generator.
}

// ErrUnknownLang is reported by Generate when the language is not known.
var ErrUnknownLang = errors.New("unknown language")

// quoteJSString renders s as a double-quoted JavaScript/TypeScript string
// literal, escaping every character that could otherwise break out of the
// literal and inject code into the generated client. Both attacker-influenced
// values (e.g. header/query wire names) and ordinary identifiers flow through
// here, so it must produce valid JS for arbitrary input.
func quoteJSString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\u2028':
			// Line separator: valid in JS string literals only since ES2019.
			b.WriteString(`\u2028`)
		case '\u2029':
			// Paragraph separator: valid in JS string literals only since ES2019.
			b.WriteString(`\u2029`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// escapeDocComment neutralizes any "*/" sequence in a line destined for a
// generated /* ... */ block comment so it cannot terminate the comment early
// and turn the following text into executable code.
func escapeDocComment(line string) string {
	return strings.ReplaceAll(line, "*/", "*\\/")
}

// Detect attempts to detect the language from the given filename.
func Detect(path string) (lang Lang, ok bool) {
	suffix := strings.ToLower(filepath.Ext(path))
	switch suffix {
	case ".ts":
		return LangTypeScript, true
	case ".js":
		return LangJavascript, true
	case ".go":
		return LangGo, true
	default:
		return LangUnknown, false
	}
}

// Client generates an API client based on the given app metadata.
// ServiceNames are the services to include in the output.
// If it's nil, all services are included.
func Client(
	lang Lang,
	appSlug string,
	md *meta.Data,
	services clientgentypes.ServiceSet,
	tags clientgentypes.TagSet,
	opts clientgentypes.Options,
) (code []byte, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = srcerrors.UnhandledPanic(e)
		}
	}()

	var gen generator
	switch lang {
	case LangTypeScript:
		if opts.TSSharedTypes && md.Language == meta.Lang_TYPESCRIPT {
			gen = &typescript{generatorVersion: typescriptGenLatestVersion, sharedTypes: true, clientTarget: opts.TSClientTarget}
		} else {
			gen = &typescript{generatorVersion: typescriptGenLatestVersion, sharedTypes: false}
		}
	case LangJavascript:
		gen = &javascript{generatorVersion: javascriptGenLatestVersion}
	case LangGo:
		gen = &golang{generatorVersion: goGenLatestVersion}
	case LangOpenAPI:
		gen = openapi.New(openapi.LatestVersion)
	default:
		return nil, ErrUnknownLang
	}

	var buf bytes.Buffer
	params := clientgentypes.GenerateParams{
		Buf:      &buf,
		AppSlug:  appSlug,
		Meta:     md,
		Services: services,
		Tags:     tags,
		Options:  opts,
	}

	if err := gen.Generate(params); err != nil {
		return nil, fmt.Errorf("genclient.Generate %s %s: %v", lang, appSlug, err)
	}
	return buf.Bytes(), nil
}

// getServiceDoc returns the documentation for a service by looking up the
// root package matching the service's rel_path
func getServiceDoc(md *meta.Data, svc *meta.Service) string {
	for _, pkg := range md.Pkgs {
		if pkg.RelPath == svc.RelPath {
			return pkg.Doc
		}
	}
	return ""
}

// GetLang returns the language specified by the given string, allowing for case insensitivity and common aliases.
func GetLang(lang string) (Lang, error) {
	switch strings.TrimSpace(strings.ToLower(lang)) {
	case "typescript", "ts":
		return LangTypeScript, nil
	case "javascript", "js":
		return LangJavascript, nil
	case "go", "golang":
		return LangGo, nil
	case "openapi", "swagger", "oas":
		return LangOpenAPI, nil
	default:
		return LangUnknown, ErrUnknownLang
	}
}
