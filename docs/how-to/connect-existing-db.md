---
seotitle: How to integrate your Encore app with an existing database
seodesc: Learn how to integrate your Encore Go backend application with an existing database, in any cloud you choose.
title: Integrate with existing databases
lang: go
---

Encore automatically provision the necessary infrastructure when you create a service and add a database. However, you may want to connect to an existing database for migration or prototyping purposes. It's simple to integrate your Encore app with an existing database in these cases.

## Example

Let's say you have an external database hosted by DigitalOcean that you would like to connect to.
The simplest approach is to create a dedicated package that lazily instantiates a database connection pool.
We can store the password using Encore's [secrets manager](/docs/develop/secrets) to make it even easier.

The connection string is something that looks like:

```
postgresql://user:password@externaldb-do-user-1234567-0.db.ondigitalocean.com:25010/externaldb?sslmode=require
```

So we write something like:

**`pkg/externaldb/externaldb.go`**

```go
package externaldb

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v4/pgxpool"
    "go4.org/syncutil"
)

// Get returns a database connection pool to the external database.
// It is lazily created on first use.
func Get(ctx context.Context) (*pgxpool.Pool, error) {
    // Attempt to setup the database connection pool if it hasn't
    // already been successfully setup.
    err := once.Do(func() error {
        var err error
        pool, err = setup(ctx)
        return err
    })
    return pool, err
}

var (
    // once is like sync.Once except it re-arms itself on failure
    once syncutil.Once
    // pool is the successfully created database connection pool,
    // or nil when no such pool has been setup yet.
    pool *pgxpool.Pool
)

var secrets struct {
    // ExternalDBPassword is the database password for authenticating
    // with the external database hosted on DigitalOcean.
    ExternalDBPassword string
}

// setup attempts to set up a database connection pool.
func setup(ctx context.Context) (*pgxpool.Pool, error) {
    connString := fmt.Sprintf("postgresql://%s:%s@externaldb-do-user-1234567-0.db.ondigitalocean.com:25010/externaldb?sslmode=require",
        "user", secrets.ExternalDBPassword)
    return pgxpool.Connect(ctx, connString)
}
```

Before running, remember to use `encore secrets set` to store the `ExternalDBPassword` to use. (But don't worry, Encore will remind you if you forget.)

## Other infrastructure

The same pattern can easily be adapted to other infrastructure components that Encore doesn't yet provide built-in support for:

- Horizontally scalable databases like Cassandra, DynamoDB, BigTable, and so on
- Document or graph databases like MongoDB or Neo4j
- Other cloud primitives like queues, object storage buckets, and more
- Or really any cloud services or APIs you can think of

In this way you can easily integrate Encore with anything you want.
