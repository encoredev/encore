package mongodb

import (
	"errors"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"encore.dev/beta/errs"
)

// IsDuplicateKey reports whether err is a duplicate key error,
// returned when a document violates a unique index (including on "_id").
func IsDuplicateKey(err error) bool {
	return mongo.IsDuplicateKeyError(err)
}

// convertErr gives "no documents" errors the errs.NotFound code.
// Other errors are returned unchanged.
//
// It must be called directly by the exported method the user called,
// so that dropping two stack frames (convertErr and that method)
// makes the error's stack start in user code.
func convertErr(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errs.DropStackFrame(errs.DropStackFrame(errs.WrapCode(err, errs.NotFound, "")))
	}
	return err
}
