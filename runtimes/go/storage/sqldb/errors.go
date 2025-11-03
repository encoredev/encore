package sqldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb/sqlerr"
)

// ErrCode reports the error code for a given error.
// If the error is nil or is not of type *Error it reports sqlerr.Other.
func ErrCode(err error) sqlerr.Code {
	var pgerr *Error
	if errors.As(err, &pgerr) {
		return pgerr.Code
	}
	return sqlerr.Other
}

// Error represents an error reported by the database server.
// It's not guaranteed all errors reported by sqldb functions will be of this type;
// it is only returned when the database reports an error.
//
// Note that the fields for schema name, table name, column name, data type name,
// and constraint name are supplied only for a limited number of error types;
// see https://www.postgresql.org/docs/current/errcodes-appendix.html.
//
// You should not assume that the presence of any of these fields guarantees
// the presence of another field.
type Error struct {
	// Code defines the general class of the error.
	// See [sqlerr.Code] for a list of possible values.
	Code sqlerr.Code

	// Severity is the severity of the error.
	Severity sqlerr.Severity

	// DatabaseCode is the database server-specific error code.
	// It is specific to the underlying database server.
	DatabaseCode string

	// Message: the primary human-readable error message. This should be accurate
	// but terse (typically one line). Always present.
	Message string

	// SchemaName: if the error was associated with a specific database object,
	// the name of the schema containing that object, if any.
	SchemaName string

	// TableName: if the error was associated with a specific table, the name of the table.
	// (Refer to the schema name field for the name of the table's schema.)
	TableName string

	// ColumnName: if the error was associated with a specific table column,
	// the name of the column. (Refer to the schema and table name fields to identify the table.)
	ColumnName string

	// Data type name: if the error was associated with a specific data type,
	// the name of the data type. (Refer to the schema name field for the name of the data type's schema.)
	DataTypeName string

	// Constraint name: if the error was associated with a specific constraint,
	// the name of the constraint. Refer to fields listed above for the associated
	// table or domain. (For this purpose, indexes are treated as constraints,
	// even if they weren't created with constraint syntax.)
	ConstraintName string

	// driverErr is the underlying error from the driver.
	// It's used to support errors.As and errors.Is to preserve
	// backwards compatibility.
	driverErr error
}

func (pe *Error) Error() string {
	return string(pe.Severity) + ": " + pe.Message + " (Code " + string(pe.Code) + ": SQLSTATE " + pe.DatabaseCode + ")"
}

func (pe *Error) Unwrap() error {
	return pe.driverErr
}

func convertErr(err error) error {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		err = convertPgError(pgerr)
	}

	switch {
	case errors.Is(err, pgx.ErrNoRows), errors.Is(err, sql.ErrNoRows):
		err = errs.WrapCode(sql.ErrNoRows, errs.NotFound, "")
	case errors.Is(err, pgx.ErrTxClosed), errors.Is(err, pgx.ErrTxCommitRollback), errors.Is(err, sql.ErrTxDone), errors.Is(err, sql.ErrConnDone):
		err = errs.WrapCode(err, errs.Internal, "")
	case errors.Is(err, context.DeadlineExceeded):
		err = errs.WrapCode(err, errs.DeadlineExceeded, "")
	case errors.Is(err, context.Canceled):
		err = errs.WrapCode(err, errs.Canceled, "")
	default:
		err = errs.WrapCode(err, errs.Unavailable, "")
	}

	return errs.DropStackFrame(err)
}

func convertPgError(src *pgconn.PgError) error {
	return &Error{
		Code:           sqlerr.MapCode(src.Code),
		Severity:       sqlerr.MapSeverity(src.Severity),
		DatabaseCode:   src.Code,
		Message:        src.Message,
		SchemaName:     src.SchemaName,
		TableName:      src.TableName,
		ColumnName:     src.ColumnName,
		DataTypeName:   src.DataTypeName,
		ConstraintName: src.ConstraintName,
		driverErr:      src,
	}
}
