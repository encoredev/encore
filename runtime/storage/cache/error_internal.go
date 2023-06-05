package cache

import (
	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/apisdk/api/errmarshalling"
)

// Register an internal error marshaller for the cache.OpError
func init() {
	errmarshalling.RegisterErrorMarshaller(
		func(err *OpError, stream *jsoniter.Stream) {
			stream.WriteObjectField("op")
			stream.WriteString(err.Operation)

			stream.WriteMore()
			stream.WriteObjectField("raw_key")
			stream.WriteString(err.RawKey)

			if err.Err != nil {
				stream.WriteMore()
				stream.WriteObjectField(errmarshalling.WrappedKey)
				stream.WriteVal(err.Err)
			}
		},
		func(err *OpError, iter *jsoniter.Iterator) {
			iter.ReadObjectCB(func(iter *jsoniter.Iterator, field string) bool {
				switch field {
				case "op":
					err.Operation = iter.ReadString()
				case "raw_key":
					err.RawKey = iter.ReadString()
				case errmarshalling.WrappedKey:
					err.Err = errmarshalling.UnmarshalError(iter)
				default:
					iter.Skip()
				}
				return true
			})
		},
	)
}
