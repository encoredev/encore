import "encoding/base64"

ReadOnlyMode: true
PublicKey: base64.Decode(null, "aGVsbG8gd29ybGQK") // "hello world" in Base64

SubConfig: SubKey: MaxCount: [
	if #Meta.Environment.Type  == "test"  { 3 },
	if #Meta.Environment.Cloud == "local" { 2 },
	1
][0]

AdminUsers: [
	"foo",
	"bar",
]
