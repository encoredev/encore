package sqldb

import (
	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/apisdk/api/errmarshalling"
	"encore.dev/storage/sqldb/sqlerr"
)

// Register an internal error marshaller for the sqldb.Error
func init() {
	errmarshalling.RegisterErrorMarshaller(
		func(err *Error, stream *jsoniter.Stream) {
			stream.WriteObjectField("code")
			stream.WriteString(string(err.Code))

			stream.WriteMore()
			stream.WriteObjectField("severity")
			stream.WriteString(string(err.Severity))

			stream.WriteMore()
			stream.WriteObjectField("db_code")
			stream.WriteString(err.DatabaseCode)

			stream.WriteMore()
			stream.WriteObjectField(errmarshalling.MessageKey)
			stream.WriteString(err.Message)

			if err.SchemaName != "" {
				stream.WriteMore()
				stream.WriteObjectField("schema")
				stream.WriteString(err.SchemaName)
			}

			if err.TableName != "" {
				stream.WriteMore()
				stream.WriteObjectField("table")
				stream.WriteString(err.TableName)
			}

			if err.ColumnName != "" {
				stream.WriteMore()
				stream.WriteObjectField("column")
				stream.WriteString(err.ColumnName)
			}

			if err.DataTypeName != "" {
				stream.WriteMore()
				stream.WriteObjectField("data_type")
				stream.WriteString(err.DataTypeName)
			}

			if err.ConstraintName != "" {
				stream.WriteMore()
				stream.WriteObjectField("constraint")
				stream.WriteString(err.ConstraintName)
			}

			if err.driverErr != nil {
				stream.WriteMore()
				stream.WriteObjectField(errmarshalling.WrappedKey)
				stream.WriteVal(err.driverErr)
			}
		},
		func(err *Error, iter *jsoniter.Iterator) {
			iter.ReadObjectCB(func(iter *jsoniter.Iterator, field string) bool {
				switch field {
				case "code":
					err.Code = sqlerr.Code(iter.ReadString())
				case "severity":
					err.Severity = sqlerr.Severity(iter.ReadString())
				case "db_code":
					err.DatabaseCode = iter.ReadString()
				case errmarshalling.MessageKey:
					err.Message = iter.ReadString()
				case "schema":
					err.SchemaName = iter.ReadString()
				case "table":
					err.TableName = iter.ReadString()
				case "column":
					err.ColumnName = iter.ReadString()
				case "data_type":
					err.DataTypeName = iter.ReadString()
				case "constraint":
					err.ConstraintName = iter.ReadString()
				case errmarshalling.WrappedKey:
					err.driverErr = errmarshalling.UnmarshalError(iter)
				default:
					iter.Skip()
				}
				return true
			})
		},
	)
}
