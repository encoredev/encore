// Package mongodb provides Encore applications with the ability
// to declare and use MongoDB databases.
//
// Declare a database as a package level variable:
//
//	var DB = mongodb.NewDatabase("urls", mongodb.DatabaseConfig{})
//
// Encore provisions the database and connects to it. Use it through
// collections, with the official driver's types for filters and results:
//
//	_, err := DB.Collection("urls").InsertOne(ctx, bson.M{"_id": id, "url": url})
//	err = DB.Collection("urls").FindOne(ctx, bson.M{"_id": id}).Decode(&u)
//
// For anything this package does not provide, use (*Database).Driver
// to get the driver's *mongo.Database.
package mongodb
