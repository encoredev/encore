package mongodb

import (
	"errors"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"encore.dev/beta/errs"
)

// duplicateKey is MongoDB's error code for a duplicate key.
const duplicateKey = 11000

// IsDuplicateKey reports whether err is a duplicate key error,
// returned when a document violates a unique index (including on "_id").
func IsDuplicateKey(err error) bool {
	var e *Error
	if errors.As(err, &e) && e.Code == duplicateKey {
		return true
	}
	return mongo.IsDuplicateKeyError(err)
}

// convertErr converts errors from the driver:
//   - "no documents" gets the errs.NotFound code (HTTP 404)
//   - errors reported by the server become an *Error with their code and labels
//   - other errors are returned unchanged
//
// It must be called directly by the exported method the user called,
// so that dropping two stack frames (convertErr and that method)
// makes the error's stack start in user code.
func convertErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errs.DropStackFrame(errs.DropStackFrame(errs.WrapCode(err, errs.NotFound, "")))
	}
	var se mongo.ServerError
	if errors.As(err, &se) {
		e := &Error{Message: err.Error(), driverErr: err}
		if codes := se.ErrorCodes(); len(codes) > 0 {
			e.Code = codes[0]
		}
		e.Labels = errorLabels(err)
		return e
	}
	return err
}

// errorLabels returns the labels of a server error.
func errorLabels(err error) []string {
	var (
		ce  mongo.CommandError
		we  mongo.WriteException
		bwe mongo.BulkWriteException
	)
	switch {
	case errors.As(err, &ce):
		return ce.Labels
	case errors.As(err, &we):
		return we.Labels
	case errors.As(err, &bwe):
		return bwe.Labels
	}
	return nil
}
