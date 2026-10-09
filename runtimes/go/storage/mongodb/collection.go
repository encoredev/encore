package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection is a MongoDB collection.
//
// Filters, updates, documents and pipelines can be any value the BSON
// encoder accepts: structs with `bson` tags, maps, bson.M or bson.D
// (use bson.D where field order matters, e.g. a sort on several fields).
//
// Failed operations return an *Error with MongoDB's error code, or an
// error with code errs.NotFound when FindOne finds no document.
// For anything this type doesn't provide, use (*Database).Driver.
type Collection struct {
	db   *Database
	coll *mongo.Collection
	err  error // set if the collection cannot be used, e.g. errNoopDB
}

// InsertOne inserts a single document into the collection.
func (c *Collection) InsertOne(ctx context.Context, document any) (*InsertOneResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "insertOne", document)
	res, err := c.coll.InsertOne(ctx, document)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	return &InsertOneResult{InsertedID: res.InsertedID}, nil
}

// InsertMany inserts the given documents into the collection.
// The documents argument must be a slice.
func (c *Collection) InsertMany(ctx context.Context, documents any) (*InsertManyResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "insertMany", documents)
	res, err := c.coll.InsertMany(ctx, documents)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	return &InsertManyResult{InsertedIDs: res.InsertedIDs}, nil
}

// FindOne finds a single document matching the filter.
//
// If no document matches, Decode returns an error with code errs.NotFound
// that also matches mongo.ErrNoDocuments with errors.Is.
func (c *Collection) FindOne(ctx context.Context, filter any, opts ...FindOneOptions) *SingleResult {
	if c.err != nil {
		return &SingleResult{err: c.err}
	}
	o := options.FindOne()
	if len(opts) > 0 {
		if opts[0].Sort != nil {
			o.SetSort(opts[0].Sort)
		}
		if opts[0].Projection != nil {
			o.SetProjection(opts[0].Projection)
		}
		if opts[0].Skip > 0 {
			o.SetSkip(opts[0].Skip)
		}
	}
	end := c.db.traceStart(c.coll.Name(), "findOne", filter)
	res := c.coll.FindOne(ctx, filter, o)
	end(res.Err())
	return &SingleResult{res: res}
}

// Find finds the documents matching the filter.
// The returned cursor must be closed, unless (*Cursor).All is used.
func (c *Collection) Find(ctx context.Context, filter any, opts ...FindOptions) (*Cursor, error) {
	if c.err != nil {
		return nil, c.err
	}
	o := options.Find()
	if len(opts) > 0 {
		if opts[0].Sort != nil {
			o.SetSort(opts[0].Sort)
		}
		if opts[0].Projection != nil {
			o.SetProjection(opts[0].Projection)
		}
		if opts[0].Skip > 0 {
			o.SetSkip(opts[0].Skip)
		}
		if opts[0].Limit > 0 {
			o.SetLimit(opts[0].Limit)
		}
	}
	end := c.db.traceStart(c.coll.Name(), "find", filter)
	cur, err := c.coll.Find(ctx, filter, o)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	return &Cursor{std: cur}, nil
}

// UpdateOne updates a single document matching the filter.
func (c *Collection) UpdateOne(ctx context.Context, filter, update any, opts ...UpdateOptions) (*UpdateResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	o := options.UpdateOne()
	if len(opts) > 0 && opts[0].Upsert {
		o.SetUpsert(true)
	}
	end := c.db.traceStart(c.coll.Name(), "updateOne", bson.D{{Key: "filter", Value: filter}, {Key: "update", Value: update}})
	res, err := c.coll.UpdateOne(ctx, filter, update, o)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	return updateResult(res), nil
}

// UpdateMany updates all documents matching the filter.
func (c *Collection) UpdateMany(ctx context.Context, filter, update any, opts ...UpdateOptions) (*UpdateResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	o := options.UpdateMany()
	if len(opts) > 0 && opts[0].Upsert {
		o.SetUpsert(true)
	}
	end := c.db.traceStart(c.coll.Name(), "updateMany", bson.D{{Key: "filter", Value: filter}, {Key: "update", Value: update}})
	res, err := c.coll.UpdateMany(ctx, filter, update, o)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	return updateResult(res), nil
}

// ReplaceOne replaces a single document matching the filter.
func (c *Collection) ReplaceOne(ctx context.Context, filter, replacement any, opts ...UpdateOptions) (*UpdateResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	o := options.Replace()
	if len(opts) > 0 && opts[0].Upsert {
		o.SetUpsert(true)
	}
	end := c.db.traceStart(c.coll.Name(), "replaceOne", bson.D{{Key: "filter", Value: filter}, {Key: "replacement", Value: replacement}})
	res, err := c.coll.ReplaceOne(ctx, filter, replacement, o)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	return updateResult(res), nil
}

// DeleteOne deletes a single document matching the filter.
func (c *Collection) DeleteOne(ctx context.Context, filter any) (*DeleteResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "deleteOne", filter)
	res, err := c.coll.DeleteOne(ctx, filter)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	return &DeleteResult{DeletedCount: res.DeletedCount}, nil
}

// DeleteMany deletes all documents matching the filter.
func (c *Collection) DeleteMany(ctx context.Context, filter any) (*DeleteResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "deleteMany", filter)
	res, err := c.coll.DeleteMany(ctx, filter)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	return &DeleteResult{DeletedCount: res.DeletedCount}, nil
}

// CountDocuments counts the documents matching the filter.
func (c *Collection) CountDocuments(ctx context.Context, filter any) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "countDocuments", filter)
	n, err := c.coll.CountDocuments(ctx, filter)
	end(err)
	return n, convertErr(err)
}

// Aggregate runs an aggregation pipeline on the collection.
// The returned cursor must be closed, unless (*Cursor).All is used.
func (c *Collection) Aggregate(ctx context.Context, pipeline any) (*Cursor, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "aggregate", pipeline)
	cur, err := c.coll.Aggregate(ctx, pipeline)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	return &Cursor{std: cur}, nil
}

// CreateIndex creates an index on the given fields and returns its name,
// e.g. CreateIndex(ctx, bson.D{{"email", 1}}, IndexOptions{Unique: true}).
// Use bson.D for keys, as the order of the fields matters.
// Creating an index that already exists does nothing.
func (c *Collection) CreateIndex(ctx context.Context, keys any, opts ...IndexOptions) (string, error) {
	if c.err != nil {
		return "", c.err
	}
	o := options.Index()
	if len(opts) > 0 {
		if opts[0].Name != "" {
			o.SetName(opts[0].Name)
		}
		if opts[0].Unique {
			o.SetUnique(true)
		}
		if opts[0].Sparse {
			o.SetSparse(true)
		}
		if opts[0].ExpireAfterSeconds != nil {
			o.SetExpireAfterSeconds(*opts[0].ExpireAfterSeconds)
		}
	}
	end := c.db.traceStart(c.coll.Name(), "createIndex", keys)
	name, err := c.coll.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: keys, Options: o})
	end(err)
	return name, convertErr(err)
}

// ListIndexes lists the indexes on the collection.
func (c *Collection) ListIndexes(ctx context.Context) ([]IndexInfo, error) {
	if c.err != nil {
		return nil, c.err
	}
	end := c.db.traceStart(c.coll.Name(), "listIndexes", nil)
	specs, err := c.coll.Indexes().ListSpecifications(ctx)
	end(err)
	if err != nil {
		return nil, convertErr(err)
	}
	out := make([]IndexInfo, 0, len(specs))
	for _, s := range specs {
		info := IndexInfo{
			Name:               s.Name,
			Unique:             s.Unique != nil && *s.Unique,
			Sparse:             s.Sparse != nil && *s.Sparse,
			ExpireAfterSeconds: s.ExpireAfterSeconds,
		}
		var keys bson.D
		if err := bson.Unmarshal(s.KeysDocument, &keys); err != nil {
			return nil, fmt.Errorf("mongodb: decode keys of index %s: %w", s.Name, err)
		}
		for _, k := range keys {
			info.Keys = append(info.Keys, IndexField{Field: k.Key, Value: k.Value})
		}
		out = append(out, info)
	}
	return out, nil
}

// DropIndex drops the index with the given name.
func (c *Collection) DropIndex(ctx context.Context, name string) error {
	if c.err != nil {
		return c.err
	}
	end := c.db.traceStart(c.coll.Name(), "dropIndex", name)
	err := c.coll.Indexes().DropOne(ctx, name)
	end(err)
	return convertErr(err)
}

func updateResult(res *mongo.UpdateResult) *UpdateResult {
	return &UpdateResult{
		MatchedCount:  res.MatchedCount,
		ModifiedCount: res.ModifiedCount,
		UpsertedCount: res.UpsertedCount,
		UpsertedID:    res.UpsertedID,
	}
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
