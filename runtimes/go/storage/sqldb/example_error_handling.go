package sqldb_test

import (
    "context"
    "errors"
    "fmt"

    "encore.dev/storage/sqldb"
    "encore.dev/storage/sqldb/sqlerr"
)

func Example() {
    ctx := context.Background()
    db := sqldb.MustConnect("postgres://user:pass@localhost/dbname")

    err := db.Exec(ctx, `INSERT INTO users(id) VALUES (1)`)

    // Check mapped error code
    if sqldb.ErrCode(err) == sqlerr.UniqueViolation {
        fmt.Println("Unique violation detected!")
    }

    // Access raw PostgreSQL error code
    var pgErr *sqldb.Error
    if errors.As(err, &pgErr) {
        fmt.Println("PostgreSQL code:", pgErr.DatabaseCode)
    }
}
