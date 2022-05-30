// Package codegen generates code for use with Encore apps.
package codegen

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Lang represents a programming language or dialect that we support generating code for.
type Lang string

// These constants represent supported languages.
const (
	LangUnknown    Lang = ""
	LangTypeScript Lang = "typescript"
	LangGo         Lang = "go"
)

type generator interface {
	Generate(buf *bytes.Buffer, appSlug string, md *meta.Data) error
	Version() int // The version of the generator.
}

// ErrUnknownLang is reported by Generate when the language is not known.
var ErrUnknownLang = errors.New("unknown language")

// Detect attempts to detect the language from the given filename.
func Detect(path string) (lang Lang, ok bool) {
	suffix := strings.ToLower(filepath.Ext(path))
	switch suffix {
	case ".ts":
		return LangTypeScript, true
	case ".go":
		return LangGo, true
	default:
		return LangUnknown, false
	}
}

// Client generates an API client based on the given app metadata.
func Client(lang Lang, appSlug string, md *meta.Data) (code []byte, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("codegen.Client %s %s panicked: %v\n%s", lang, appSlug, e, debug.Stack())
		}
	}()

	var gen generator
	switch Lang(strings.ToLower(string(lang))) {
	case LangTypeScript:
		gen = &typescript{generatorVersion: typescriptGenLatestVersion}
	case LangGo:
		gen = &golang{generatorVersion: goGenLatestVersion}
	default:
		return nil, ErrUnknownLang
	}

	var buf bytes.Buffer
	if err := gen.Generate(&buf, appSlug, md); err != nil {
		return nil, fmt.Errorf("genclient.Generate %s %s: %v", lang, appSlug, err)
	}
	return buf.Bytes(), nil
}
