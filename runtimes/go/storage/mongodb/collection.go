package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection is a MongoDB collection.
//
// Its methods are thin wrappers around the official driver's
// *mongo.Collection, and take and return the driver's types.
// See https://pkg.go.dev/go.mongodb.org/mongo-driver/v2/mongo#Collection
// for additional documentation.
type Collection struct {
	db   *Database
	coll *mongo.Collection
	err  error // set if the collection cannot be used, e.g. errNoopDB
}

// InsertOne inserts a single document into the collection.
func (c *Collection) InsertOne(ctx context.Context, document any, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "insertOne", document)
	res, err := c.coll.InsertOne(ctx, document, opts...)
	end(err)
	return res, err
}

// InsertMany inserts the given documents into the collection.
// The documents argument must be a slice.
func (c *Collection) InsertMany(ctx context.Context, documents any, opts ...options.Lister[options.InsertManyOptions]) (*mongo.InsertManyResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "insertMany", documents)
	res, err := c.coll.InsertMany(ctx, documents, opts...)
	end(err)
	return res, err
}

// FindOne finds a single document matching the filter.
//
// If no document matches, Decode returns an error with code errs.NotFound
// that also matches mongo.ErrNoDocuments with errors.Is.
func (c *Collection) FindOne(ctx context.Context, filter any, opts ...options.Lister[options.FindOneOptions]) *SingleResult {
	if c.err != nil {
		return &SingleResult{err: c.err}
	}
	end := c.db.traceStart(c.coll.Name(), "findOne", filter)
	res := c.coll.FindOne(ctx, filter, opts...)
	end(res.Err())
	return &SingleResult{res: res}
}

// Find finds the documents matching the filter.
func (c *Collection) Find(ctx context.Context, filter any, opts ...options.Lister[options.FindOptions]) (*mongo.Cursor, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "find", filter)
	res, err := c.coll.Find(ctx, filter, opts...)
	end(err)
	return res, err
}

// UpdateOne updates a single document matching the filter.
func (c *Collection) UpdateOne(ctx context.Context, filter, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "updateOne", bson.D{{Key: "filter", Value: filter}, {Key: "update", Value: update}})
	res, err := c.coll.UpdateOne(ctx, filter, update, opts...)
	end(err)
	return res, err
}

// UpdateMany updates all documents matching the filter.
func (c *Collection) UpdateMany(ctx context.Context, filter, update any, opts ...options.Lister[options.UpdateManyOptions]) (*mongo.UpdateResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "updateMany", bson.D{{Key: "filter", Value: filter}, {Key: "update", Value: update}})
	res, err := c.coll.UpdateMany(ctx, filter, update, opts...)
	end(err)
	return res, err
}

// ReplaceOne replaces a single document matching the filter.
func (c *Collection) ReplaceOne(ctx context.Context, filter, replacement any, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "replaceOne", bson.D{{Key: "filter", Value: filter}, {Key: "replacement", Value: replacement}})
	res, err := c.coll.ReplaceOne(ctx, filter, replacement, opts...)
	end(err)
	return res, err
}

// DeleteOne deletes a single document matching the filter.
func (c *Collection) DeleteOne(ctx context.Context, filter any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "deleteOne", filter)
	res, err := c.coll.DeleteOne(ctx, filter, opts...)
	end(err)
	return res, err
}

// DeleteMany deletes all documents matching the filter.
func (c *Collection) DeleteMany(ctx context.Context, filter any, opts ...options.Lister[options.DeleteManyOptions]) (*mongo.DeleteResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "deleteMany", filter)
	res, err := c.coll.DeleteMany(ctx, filter, opts...)
	end(err)
	return res, err
}

// CountDocuments counts the documents matching the filter.
func (c *Collection) CountDocuments(ctx context.Context, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "countDocuments", filter)
	res, err := c.coll.CountDocuments(ctx, filter, opts...)
	end(err)
	return res, err
}

// Aggregate runs an aggregation pipeline on the collection.
func (c *Collection) Aggregate(ctx context.Context, pipeline any, opts ...options.Lister[options.AggregateOptions]) (*mongo.Cursor, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "aggregate", pipeline)
	res, err := c.coll.Aggregate(ctx, pipeline, opts...)
	end(err)
	return res, err
}

// Indexes returns the index view for the collection,
// for creating and listing indexes.
func (c *Collection) Indexes() mongo.IndexView {
	if c.err != nil {
		return mongo.IndexView{}
	}
	return c.coll.Indexes()
}

// SingleResult is the result of FindOne.
type SingleResult struct {
	res *mongo.SingleResult
	err error
}

// Decode decodes the found document into v.
//
// If no document was found it returns an error with code errs.NotFound,
// so an API endpoint returning it responds with HTTP 404.
func (r *SingleResult) Decode(v any) error {
	if r.err != nil {
		return r.err
	}
	return convertErr(r.res.Decode(v))
}

// Err reports the error of the FindOne operation, if any.
// It reports errs.NotFound if no document was found.
func (r *SingleResult) Err() error {
	if r.err != nil {
		return r.err
	}
	return convertErr(r.res.Err())
}
