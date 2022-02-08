package pgproto3

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"encr.dev/pkg/pgproxy2/internal/pgio"
)

const (
	ProtocolVersionNumber = 196608 // 3.0
)

type StartupMessage struct {
	ProtocolVersion uint32
	Parameters      map[string]string
}

func (*StartupMessage) Frontend() {}

func (dst *StartupMessage) Decode(src []byte) error {
	if len(src) < 4 {
		return errors.New("startup message too short")
	}

	dst.ProtocolVersion = binary.BigEndian.Uint32(src)
	rp := 4

	if dst.ProtocolVersion == sslRequestNumber {
		return errors.New("can't handle ssl connection request")
	}

	if dst.ProtocolVersion != ProtocolVersionNumber {
		return fmt.Errorf("Bad startup message version number. Expected %d, got %d", ProtocolVersionNumber, dst.ProtocolVersion)
	}

	dst.Parameters = make(map[string]string)
	for {
		idx := bytes.IndexByte(src[rp:], 0)
		if idx < 0 {
			return &invalidMessageFormatErr{messageType: "StartupMesage"}
		}
		key := string(src[rp : rp+idx])
		rp += idx + 1

		idx = bytes.IndexByte(src[rp:], 0)
		if idx < 0 {
			return &invalidMessageFormatErr{messageType: "StartupMesage"}
		}
		value := string(src[rp : rp+idx])
		rp += idx + 1

		dst.Parameters[key] = value

		if len(src[rp:]) == 1 {
			if src[rp] != 0 {
				return fmt.Errorf("Bad startup message last byte. Expected 0, got %d", src[rp])
			}
			break
		}
	}

	return nil
}

func (src *StartupMessage) Encode(dst []byte) []byte {
	sp := len(dst)
	dst = pgio.AppendInt32(dst, -1)

	dst = pgio.AppendUint32(dst, src.ProtocolVersion)
	for k, v := range src.Parameters {
		dst = append(dst, k...)
		dst = append(dst, 0)
		dst = append(dst, v...)
		dst = append(dst, 0)
	}
	dst = append(dst, 0)

	pgio.SetInt32(dst[sp:], int32(len(dst[sp:])))

	return dst
}

func (src *StartupMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type            string
		ProtocolVersion uint32
		Parameters      map[string]string
	}{
		Type:            "StartupMessage",
		ProtocolVersion: src.ProtocolVersion,
		Parameters:      src.Parameters,
	})
}
