// Package clientgen generates code for use with Encore apps.
package clientgen

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"encr.dev/internal/clientgen/clientgentypes"
	"encr.dev/internal/clientgen/openapi"
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
func Client(lang Lang, appSlug string, md *meta.Data, serviceNames []string) (code []byte, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = srcerrors.UnhandledPanic(e)
		}
	}()

	var gen generator
	switch lang {
	case LangTypeScript:
		gen = &typescript{generatorVersion: typescriptGenLatestVersion}
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
		Buf:     &buf,
		AppSlug: appSlug,
		Meta:    md,
	}
	if serviceNames == nil {
		return nil, errors.New("serviceNames must not be empty")
	}
	params.Services = clientgentypes.NewServiceSet(serviceNames...)

	if err := gen.Generate(params); err != nil {
		return nil, fmt.Errorf("genclient.Generate %s %s: %v", lang, appSlug, err)
	}
	return buf.Bytes(), nil
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
