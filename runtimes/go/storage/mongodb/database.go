package mongodb

// Database represents a MongoDB database.
//
// See NewDatabase for more information on how to declare a Database.
type Database struct {
	name string
}

// DatabaseConfig specifies configuration for declaring a new MongoDB database.
type DatabaseConfig struct{}
