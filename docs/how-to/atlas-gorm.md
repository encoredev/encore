---
seotitle: How to use Atlas + GORM for database migrations with Encore
seodesc: See how you can use Atlas to manage your database migrations in your Encore application.
title: Use Atlas + GORM for database migrations
---

[Atlas](https://atlasgo.io) is a popular tool for managing database migrations.
[GORM](https://gorm.io/) is a popular ORM for Go.

Encore provides excellent support for using them together to easily manage database schemas and migrations.
Encore executes database migrations using [golang-migrate](https://github.com/golang-migrate/migrate),
which Atlas supports out-of-the-box. This means that you can use Atlas to manage your Encore database migrations.

The easiest way to use Atlas + GORM together is with Atlas's support for [external schemas](https://atlasgo.io/blog/2023/06/28/external-schemas-and-gorm-support).

## Setting up GORM

To set up your Encore application with GORM, start by installing the GORM package and associated Postgres driver:

```shell
go get -u gorm.io/gorm gorm.io/driver/postgres
```

Then, in the service that you want to use GORM for, add the `*gorm.DB` as a dependency
in your service struct (create a service struct if you don't already have one).

For example, if you had a service called `blog`:

```go
-- blog/blog.go --
package blog

import (
	"encore.dev/storage/sqldb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)


//encore:service
type Service struct {
	db *gorm.DB
}

var blogDB = sqldb.NewDatabase("blog", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// initService initializes the site service.
// It is automatically called by Encore on service startup.
func initService() (*Service, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: blogDB.Stdlib(),
	}))
	if err != nil {
		return nil, err
	}
	return &Service{db: db}, nil
}
```

Finally, create the `migrations` directory inside the `blog` directory if it doesn't already exist.
This is where Atlas will put your database migrations.

## Setting up Atlas

First [install Atlas](https://atlasgo.io/getting-started).

Then, add an `atlas.hcl` file inside the `blog` directory:

```
-- blog/atlas.hcl --
data "external_schema" "gorm" {
  program = ["encore", "alpha", "exec", "./scripts/atlas-gorm-loader"]
}

env "local" {
  src = data.external_schema.gorm.url
  dev = "docker://postgres/15"

  migration {
    dir = "file://migrations"
    format = golang-migrate
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
```

Finally we need to create the `atlas-gorm-loader` script referenced above. We'll use Atlas's provided
[atlas-provider-gorm](https://github.com/ariga/atlas-provider-gorm) library.

```go
-- blog/scripts/atlas-gorm-loader/main.go --
package main

import (
    "fmt"
    "io"
    "os"

    _ "ariga.io/atlas-go-sdk/recordriver"
    "ariga.io/atlas-provider-gorm/gormschema"
    "encore.app/blog"
)

// Define the models to generate migrations for.
var models = []any{
    &blog.Post{},
    &blog.Comment{},
}

func main() {
    stmts, err := gormschema.New("postgres").Load(models...)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to load gorm schema: %v\n", err)
        os.Exit(1)
    }
    io.WriteString(os.Stdout, stmts)
}
```

## Creating migrations

Then, whenever you're ready to generate a migration, run:

```shell
$ atlas migrate diff --env local <name-of-migration>
```

This will generate a new migration file in the `migrations` directory, which
will be automatically applied when running `encore run`.
