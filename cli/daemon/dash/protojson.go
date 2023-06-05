package dash

import (
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type protoEncoderExtension struct {
	jsoniter.DummyExtension
	messageType reflect2.Type
	opts        protojson.MarshalOptions
}

func newProtoEncoderExtension() *protoEncoderExtension {
	msgType := reflect2.TypeOfPtr((*proto.Message)(nil)).Elem()
	opts := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}
	return &protoEncoderExtension{
		messageType: msgType,
		opts:        opts,
	}
}

func (e *protoEncoderExtension) DecorateEncoder(typ reflect2.Type, encoder jsoniter.ValEncoder) jsoniter.ValEncoder {
	if typ.Implements(e.messageType) {
		return &messageEncoder{typ: typ, encoder: encoder, opts: e.opts}
	}
	return encoder
}

type messageEncoder struct {
	typ     reflect2.Type
	encoder jsoniter.ValEncoder
	opts    protojson.MarshalOptions
}

func (codec *messageEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return codec.encoder.IsEmpty(ptr)
}

func (codec *messageEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	if msg, ok := codec.typ.UnsafeIndirect(ptr).(proto.Message); ok {
		data, err := codec.opts.Marshal(msg)
		if err != nil {
			if stream.Error == nil {
				stream.Error = err
			}
		} else {
			stream.Write(data)
		}
		return
	}
	codec.encoder.Encode(ptr, stream)
}

var protoEncoder = (func() jsoniter.API {
	enc := jsoniter.Config{}.Froze()
	// Note: the order is important. We don't want the list encoder to process repeated fields in proto
	// messages, so it must come first so it only applies to non-protobuf slices.
	enc.RegisterExtension(NewListEncoderExtension())
	enc.RegisterExtension(newProtoEncoderExtension())
	return enc
})()
