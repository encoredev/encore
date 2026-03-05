//go:build encore_app

package sqldb

func newDatabase(name string, cfg DatabaseConfig) *Database {
	return Singleton.GetDB(name)
}

func named(name constStr) *Database {
	return Singleton.GetDB(string(name))
}

func getCurrentDB() *Database {
	return Singleton.GetCurrentDB()
}
