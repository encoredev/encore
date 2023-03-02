package platform

import (
	"testing"

	qt "github.com/frankban/quicktest"
	jsoniter "github.com/json-iterator/go"

	"encr.dev/v2/cli/internal/platform/gql"
)

func TestInterfaceDecoder(t *testing.T) {
	c := qt.New(t)
	enc := jsoniter.Config{}.Froze()
	enc.RegisterExtension(NewInterfaceCodecExtension())

	data := []byte(`{
	"key": "test",
	"selector": [
		{"__typename": "SecretSelectorEnvType", "kind": "type:production"},
		{"__typename": "SecretSelectorSpecificEnv", "env": {"name": "test"}}
	]
}`)

	var group *gql.SecretGroup
	err := enc.Unmarshal(data, &group)
	c.Assert(err, qt.IsNil)
	c.Assert(group, qt.DeepEquals, &gql.SecretGroup{
		Key: "test",
		Selector: []gql.SecretSelector{
			&gql.SecretSelectorEnvType{Kind: "type:production"},
			&gql.SecretSelectorSpecificEnv{Env: &gql.Env{Name: "test"}},
		},
	})
}
