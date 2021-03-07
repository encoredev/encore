package pgproto3

import (
	"encoding/binary"
	"encoding/json"
	"errors"

	"encr.dev/pkg/pgproxy/internal/pgio"
)

const (
	sslRequestNumber = 80877103
)

type SSLMessage struct{}

func (*SSLMessage) Frontend() {}

func (dst *SSLMessage) Decode(src []byte) error {
	if len(src) != 4 {
		return errors.New("ssl message too short")
	}

	if version := binary.BigEndian.Uint32(src); version != sslRequestNumber {
		return errors.New("bad version number")
	}
	return nil
}

func (src *SSLMessage) Encode(dst []byte) []byte {
	sp := len(dst)
	dst = pgio.AppendInt32(dst, -1)

	dst = pgio.AppendUint32(dst, sslRequestNumber)
	pgio.SetInt32(dst[sp:], int32(len(dst[sp:])))
	return dst
}

func (src *SSLMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string
	}{
		Type: "SSLMessage",
	})
}
