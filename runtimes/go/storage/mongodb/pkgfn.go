//go:build encore_app

package mongodb

// NewDatabase declares a new MongoDB database.
//
// Encore uses static analysis to identify databases and their configuration,
// so all parameters passed to this function must be constant literals.
//
// A call to NewDatabase can only be made when declaring a package level variable. Any
// calls to this function made outside a package level variable declaration will result
// in a compiler error.
//
// The database name must be unique within the Encore application. Database names must be defined
// in snake_case (lowercase alphanumerics and underscore separated). Once created and deployed never
// change the database name, or else a new database will be created.
func NewDatabase(name string, config DatabaseConfig) *Database {
	// Connecting to MongoDB is not implemented yet; for now only the
	// declaration exists so that Encore can parse it.
	return &Database{name: name}
}
