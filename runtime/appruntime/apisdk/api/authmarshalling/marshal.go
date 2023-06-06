package authmarshalling

import (
	jsoniter "github.com/json-iterator/go"
)

var jsonInstance jsoniter.API

func init() {
	jsonInstance = jsoniter.Config{
		IndentionStep: 0,

		// we use a custom tag key to avoid name conflicts
		TagKey: "encore-auth-data-json",
	}.Froze()

	// We register this extension to support unexported fields.
	// so we ensure that we marshal all the data in the struct.
	// and so that we can unmarshal it back on another service.
	//
	// This is to minimise the risk of something like
	// `user.banned` not being marshalled, and the zero value
	// on the other side being `false` instead of `true`.
	jsonInstance.RegisterExtension(&unexportedFieldsSupport{})
}

// Marshal marshals the given value to a string.
func Marshal(v any) (string, error) {
	jsonBytes, err := jsonInstance.Marshal(v)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// Unmarshal unmarshals the given string into the given value.
func Unmarshal(data string, v any) error {
	return jsonInstance.Unmarshal([]byte(data), v)
}
