//go:build !encore_app

package jsonapi

// Note: This version of the file exists so we can run `go test` on the runtime module.
// This is because during those test we skip anything flagged as `encore_app` as we are
// not running inside an Encore application and so don't have access to things like
// configuration or compiled overlays.

import (
	jsoniter "github.com/json-iterator/go"
)

var Default = jsonAPI()

func jsonAPI() jsoniter.API {
	return jsoniter.Config{
		EscapeHTML:             false,
		IndentionStep:          2,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
	}.Froze()
}
