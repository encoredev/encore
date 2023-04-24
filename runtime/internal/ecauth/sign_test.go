package ecauth

import (
	"bufio"
	"bytes"
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

type TestPayload struct {
	StrMap map[string]string
	IntMap map[int]string
}

func (t *TestPayload) DeterministicBytes() []byte {
	b, _ := json.Marshal(t)
	return b
}

func Test_Sign(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	payload := &TestPayload{
		StrMap: map[string]string{"a": "z", "b": "y"},
		IntMap: map[int]string{1: "z", 2: "y"},
	}

	op, err := NewOperationHash(PubsubMsg, Read, payload, []byte("additional context"))
	c.Assert(err, qt.IsNil, qt.Commentf("got an error creating the OperationHash"))

	key := &Key{32, []byte{53, 244, 2, 73, 36, 19, 74, 222, 68, 169, 52, 68, 136, 8, 3, 227, 88, 58, 218, 84, 56, 165, 59, 181, 198, 61, 123, 98, 205, 1, 49, 124}}

	// Sign the request
	headers := Sign(key, "test-app-3d5c", "pr-34", op)

	// Run the header through the wire format to ensure that it doesn't cause an issue
	// and then parse the signing components back out of that header
	keyID, appSlug, envName, timestamp, retrievedOpHash, err := viaWireFormat(c, headers).SigningComponents()
	c.Assert(err, qt.IsNil, qt.Commentf("got an error parsing the signing components"))

	// Check that the signing components match what we expect
	c.Assert(keyID, qt.Equals, uint32(32), qt.Commentf("keyID does not match"))
	c.Assert(appSlug, qt.Equals, "test-app-3d5c", qt.Commentf("appSlug does not match"))
	c.Assert(envName, qt.Equals, "pr-34", qt.Commentf("envName does not match"))
	c.Assert(time.Since(timestamp) < time.Second, qt.Equals, true, qt.Commentf("timestamp was not now"))
	c.Assert(retrievedOpHash, qt.Equals, op, qt.Commentf("operation hash does not match"))

	// Sleep for two seconds to ensure we're using the correct timestamp
	time.Sleep(2 * time.Second)

	// Now resign the request with the retrieved signing components to check we can verify it
	// and we're deterministic in our signing.
	newHeaders := SignForVerification(key, appSlug, envName, timestamp, retrievedOpHash)
	c.Assert(newHeaders.Authorization, qt.Equals, headers.Authorization, qt.Commentf("resigned headers do not match"))
	c.Assert(newHeaders.Date, qt.Equals, headers.Date, qt.Commentf("resigned headers do not match"))
	c.Assert(newHeaders.Equal(headers), qt.Equals, true, qt.Commentf("equals method reported wrong result"))
}

// viaWireFormat is a hack to ensure that the headers are marshalled in the same way as they would be over the wire.
// and then unmarshalled back, making sure that the wireformat doesn't cause an issue with the signing.
func viaWireFormat(c *qt.C, headers *Headers) *Headers {
	httpHeaders := make(http.Header)
	httpHeaders.Set("Authorization", headers.Authorization)
	httpHeaders.Set("Date", headers.Date)

	// Write an HTTP request to a buffer, which includes the headers
	var buf bytes.Buffer
	buf.Write([]byte(
		"GET / HTTP/1.1\r\n" +
			"Host: foo.com\r\n",
	))
	c.Assert(httpHeaders.Write(&buf), qt.IsNil, qt.Commentf("got an error writing the headers"))
	buf.Write([]byte("\r\n"))

	request, err := http.ReadRequest(bufio.NewReader(&buf))
	c.Assert(err, qt.IsNil, qt.Commentf("got an error reading the request"))

	return &Headers{
		Authorization: request.Header.Get("Authorization"),
		Date:          request.Header.Get("Date"),
	}
}
