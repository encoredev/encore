//go:build !encore_app

package sqldb

func newDatabase(name string, cfg DatabaseConfig) *Database {
	if true {
		panic("only implemented at app runtime")
	}
	return nil
}

func named(name constStr) *Database {
	if true {
		panic("only implemented at app runtime")
	}
	return nil
}

func getCurrentDB() *Database {
	if true {
		panic("only implemented at app runtime")
	}
	return nil
}
