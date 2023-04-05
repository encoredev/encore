package jsonapi

import (
	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/appconf"
)

var Default = jsonAPI(appconf.Runtime)

func jsonAPI(rt *config.Runtime) jsoniter.API {
	indentStep := 2
	if rt.EnvType == "production" {
		indentStep = 0
	}
	return jsoniter.Config{
		EscapeHTML:             false,
		IndentionStep:          indentStep,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
	}.Froze()
}
