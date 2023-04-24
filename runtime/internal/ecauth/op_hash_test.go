package ecauth

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNewOperationHash(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	tests := []struct {
		name              string
		object            ObjectType
		action            ActionType
		payload           Payload
		additionalContext [][]byte
		want              OperationHash
		wantErr           string
	}{
		{name: "basic", object: PubsubMsg, action: Create, want: "fecc54807aa7f2397b89ee7f1bbcb9eee043621dd546633168c9b3c49ddc4e65"},
		{name: "changed action", object: PubsubMsg, action: Read, want: "ad4504cf4110741ac9651fa0bb67918e1b8c48627ce5680959922c7c0c4426bc"},
		{name: "changed object", object: "RPC", action: Read, want: "3041704d18c6eb2d13b8e1b52d55b5737e723c3a4549aca1b637edb15f94a43c"},
		{name: "with additional", object: PubsubMsg, action: Create, additionalContext: [][]byte{{1, 99, 11, 43, 83}}, want: "0ff48aa9c8c00520e0a8a347ebbf5bf384f025f21d5c3b97ac36802439783639"},
		{name: "with additional split", object: PubsubMsg, action: Create, additionalContext: [][]byte{{1, 99, 11}, {43, 83}}, want: "e599a3aefc6cf42a6bb4d3c791f409ada4176059c280b1652ce2747ed9822d26"},
		{name: "with payload", object: PubsubMsg, action: Create, payload: &TestPayload{StrMap: map[string]string{"a": "1", "c": "1"}}, want: "a0218534ee9a02b6db9b23099f2970116f9c5268374794271147870192e6484b"},
		{name: "with payload different order", object: PubsubMsg, action: Create, payload: &TestPayload{StrMap: map[string]string{"c": "1", "a": "1"}}, want: "a0218534ee9a02b6db9b23099f2970116f9c5268374794271147870192e6484b"},
		{name: "with payload different value", object: PubsubMsg, action: Create, payload: &TestPayload{StrMap: map[string]string{"c": "2", "a": "1"}}, want: "1a5647f5c34be98caab0f264f808d8e1bc23a6cb323ecc1b62bc9db48025025d"},
		{name: "with payload and additional", object: PubsubMsg, action: Create, additionalContext: [][]byte{{1, 99, 11, 43, 83}}, payload: &TestPayload{StrMap: map[string]string{"c": "1", "a": "1"}}, want: "6e4139808b3a68c6fb7289c46e8e3a4a1cacbf1e794156b8936373627101ca8f"},
	}
	for _, tt := range tests {
		tt := tt
		c.Run(tt.name, func(c *qt.C) {
			c.Parallel()

			// Test the generation of the operation hash is deterministic
			got, err := NewOperationHash(tt.object, tt.action, tt.payload, tt.additionalContext...)
			if tt.wantErr != "" {
				c.Assert(err, qt.ErrorMatches, tt.wantErr)
				return
			}
			c.Assert(err, qt.IsNil, qt.Commentf("got an unexpected error generation the operation hash"))
			c.Assert(got, qt.Equals, tt.want, qt.Commentf("OpHash was not as expected"))

			// Test the verify function works
			ok, err := got.Verify(tt.object, tt.action, tt.payload, tt.additionalContext...)
			c.Assert(err, qt.IsNil, qt.Commentf("got an unexpected error verifying the operation hash"))
			c.Assert(ok, qt.IsTrue, qt.Commentf("OpHash was not verified"))

			// Test if we change something it fails to verify
			ok, err = got.Verify(tt.object, "another", tt.payload, tt.additionalContext...)
			c.Assert(err, qt.IsNil, qt.Commentf("got an unexpected error verifying the operation hash"))
			c.Assert(ok, qt.IsFalse, qt.Commentf("OpHash was not verified"))
		})
	}
}
