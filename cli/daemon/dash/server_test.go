package dash

import (
	"bytes"
	"encoding/json"
	"testing"

	"encr.dev/cli/internal/jsonrpc2"
)

func TestServer_notify(t *testing.T) {
	t.Run("encodes once and sends every client the same message", func(t *testing.T) {
		s := &Server{clients: make(map[chan<- []byte]struct{})}
		var chans []chan []byte
		for range 3 {
			ch := make(chan []byte, 1)
			s.addClient(ch)
			chans = append(chans, ch)
		}
		params := &countingParams{Name: "app"}

		s.notify(&notification{Method: "process/reload", Params: params})

		if params.encoded != 1 {
			t.Errorf("params encoded %d times, want 1", params.encoded)
		}
		first := <-chans[0]
		for _, ch := range chans[1:] {
			if got := <-ch; !bytes.Equal(got, first) {
				t.Errorf("clients got different messages: %s and %s", first, got)
			}
		}
		msg, err := jsonrpc2.DecodeMessage(first)
		if err != nil {
			t.Fatalf("decode %s: %v", first, err)
		}
		n, ok := msg.(*jsonrpc2.Notification)
		if !ok {
			t.Fatalf("got a %T, want a notification", msg)
		}
		if n.Method() != "process/reload" || string(n.Params()) != `{"name":"app"}` {
			t.Errorf("got method %q with params %q", n.Method(), n.Params())
		}
	})

	t.Run("encodes nothing with no client", func(t *testing.T) {
		s := &Server{clients: make(map[chan<- []byte]struct{})}
		params := &countingParams{}

		s.notify(&notification{Method: "process/reload", Params: params})

		if params.encoded != 0 {
			t.Errorf("params encoded %d times, want 0", params.encoded)
		}
	})
}

// countingParams counts how often it is encoded.
type countingParams struct {
	Name    string `json:"name"`
	encoded int
}

func (p *countingParams) MarshalJSON() ([]byte, error) {
	p.encoded++
	return json.Marshal(struct {
		Name string `json:"name"`
	}{p.Name})
}
