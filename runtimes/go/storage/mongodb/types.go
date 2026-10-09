package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// InsertOneResult is the result of InsertOne.
type InsertOneResult struct {
	// InsertedID is the _id of the inserted document.
	InsertedID any
}

// InsertManyResult is the result of InsertMany.
type InsertManyResult struct {
	// InsertedIDs are the _ids of the inserted documents, in order.
	InsertedIDs []any
}

// UpdateResult is the result of UpdateOne, UpdateMany and ReplaceOne.
type UpdateResult struct {
	// MatchedCount is the number of documents that matched the filter.
	MatchedCount int64
	// ModifiedCount is the number of documents that were modified.
	ModifiedCount int64
	// UpsertedCount is the number of documents inserted by an upsert (0 or 1).
	UpsertedCount int64
	// UpsertedID is the _id of the document inserted by an upsert, or nil.
	UpsertedID any
}

// DeleteResult is the result of DeleteOne and DeleteMany.
type DeleteResult struct {
	// DeletedCount is the number of documents that were deleted.
	DeletedCount int64
}

// FindOptions are options for Find.
type FindOptions struct {
	// Sort is the order to sort matching documents in, e.g. bson.D{{"age", -1}}.
	// Use an ordered type (bson.D) to sort by more than one field.
	Sort any
	// Projection is the fields to include or exclude, e.g. bson.M{"name": 1}.
	Projection any
	// Skip is the number of matching documents to skip.
	Skip int64
	// Limit is the maximum number of documents to return. 0 means no limit.
	Limit int64
}

// FindOneOptions are options for FindOne.
type FindOneOptions struct {
	// Sort decides which document is returned when several match.
	Sort any
	// Projection is the fields to include or exclude.
	Projection any
	// Skip is the number of matching documents to skip.
	Skip int64
}

// UpdateOptions are options for UpdateOne, UpdateMany and ReplaceOne.
type UpdateOptions struct {
	// Upsert inserts a new document if no document matches the filter.
	Upsert bool
}

// IndexOptions are options for CreateIndex.
type IndexOptions struct {
	// Name is the name of the index. MongoDB generates one if empty.
	Name string
	// Unique rejects documents whose indexed fields duplicate another document's.
	Unique bool
	// Sparse only indexes documents that have the indexed fields.
	Sparse bool
	// ExpireAfterSeconds, if set, deletes documents this many seconds
	// after the date in the indexed field (a TTL index).
	ExpireAfterSeconds *int32
}

// IndexField is one field of an index, in order.
type IndexField struct {
	// Field is the name of the indexed field.
	Field string
	// Value is the index type: 1 (ascending), -1 (descending), or e.g. "text".
	Value any
}

// IndexInfo describes an index on a collection.
type IndexInfo struct {
	Name               string
	Keys               []IndexField
	Unique             bool
	Sparse             bool
	ExpireAfterSeconds *int32
}

// Cursor iterates over the documents returned by Find or Aggregate.
//
// It must be closed when done, unless All was called:
//
//	cur, err := coll.Find(ctx, filter)
//	if err != nil { ... }
//	defer cur.Close(ctx)
//	for cur.Next(ctx) {
//		var doc T
//		if err := cur.Decode(&doc); err != nil { ... }
//	}
//	if err := cur.Err(); err != nil { ... }
type Cursor struct {
	std *mongo.Cursor
}

// Next moves to the next document. It reports false when there are no
// more documents or an error occurred; check Err to tell them apart.
func (c *Cursor) Next(ctx context.Context) bool {
	return c.std.Next(ctx)
}

// Decode decodes the current document into v.
func (c *Cursor) Decode(v any) error {
	return convertErr(c.std.Decode(v))
}

// All decodes every remaining document into results, which must be a
// pointer to a slice, and closes the cursor.
func (c *Cursor) All(ctx context.Context, results any) error {
	return convertErr(c.std.All(ctx, results))
}

// Err reports the error, if any, that stopped Next.
func (c *Cursor) Err() error {
	return convertErr(c.std.Err())
}

// Close closes the cursor.
func (c *Cursor) Close(ctx context.Context) error {
	return convertErr(c.std.Close(ctx))
}

// Error is an error reported by MongoDB.
//
// The driver's original error stays available with errors.As and errors.Is.
type Error struct {
	// Code is the MongoDB error code, e.g. 11000 for a duplicate key.
	Code int
	// Labels are the MongoDB error labels, e.g. "TransientTransactionError".
	Labels []string
	// Message is the error message.
	Message string

	driverErr error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.driverErr
}

// HasErrorLabel reports whether the error has the given label.
func (e *Error) HasErrorLabel(label string) bool {
	for _, l := range e.Labels {
		if l == label {
			return true
		}
	}
	return false
}
