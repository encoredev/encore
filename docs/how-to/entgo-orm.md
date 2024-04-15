---
seotitle: Use ent + Atlas for database schema management with Encore.
seodesc: See how you can use an ORM like ent with Atlas to handle your database schemas.
title: Use ent ORM + Atlas for database schemas
lang: go
---

Encore has all the tools needed to support ORMs and migration frameworks out-of-the-box through
[named databases](/docs/how-to/share-db-between-services) and 
[migration files](/docs/develop/databases#defining-a-database-schema). Writing plain SQL might
not work for your use case, or you may not want to use SQL in the first place. 

ORMs like [ent](https://entgo.io/) and migration frameworks like [Atlas](https://atlasgo.io/) can
be used with Encore by integrating their logic with a system's database. Encore is not restrictive,
it uses plain SQL migration files for its migrations. 

- If your ORM of choice can connect to any database using a [standard SQL driver](https://github.com/lib/pq), then it can be used with Encore.
- If your migration framework can generate SQL migration files without any modifications, then it can be used with Encore.

Let's take a look at how you can integrate ent with Encore, using Atlas for generating the migration files.

## Add ent schemas to a service
[Install ent](https://entgo.io/docs/tutorial-setup#installation), then initialize your first
schema in the service where you want to use it. For example, if you had the following app structure:

```
/my-app
├── encore.app
└── user        // user service
```

You can then use this command to generate a user schema along with the ent directory that will contain
that schema and all future generated files:

```shell
$ go run entgo.io/ent/cmd/ent@latest new --target user/ent/schema User
```

The `--target` option sets the schema directory within your Encore system. Each system
should contain its own models and schemas, and its own migration files. Like you would when using
plain SQL.

Add the fields and edges for your new model in the generated file under `user/ent/schema/user.go`.

Now, run the following command:

```shell
$ go run entgo.io/ent/cmd/ent@latest generate ./user/ent/schema
```

This generates the ent client files. Run this command again whenever you change the schemas.

## Integrating ent with an Encore database

Encore automates database provisioning, and automatically runs migrations in all environments.

To integrate ent with Encore, we need to do three things:

1. Create the Encore database
2. Set up the ent client to use that database.
3. Generate migration files for the ent schema, using Atlas.

### Create the Encore database

Create the database using [`sqldb.NewDatabase`](/docs/primitives/databases) in `user/user.go`:

```
-- user/user.go --
package user

import "encore.dev/storage/sqldb"

var userDB = sqldb.NewDatabase("user", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})
```

Now, create the `migrations` directory, and leave it empty for now:

```shell
$ mkdir user/migrations
```

### Connect ent to the database

Next, extend the user service with a [Service Struct](/docs/primitives/services-and-apis/service-structs) that
creates an ent client connected to the database.

Replace the contents of the `user/user.go` file with:

```
-- user/user.go --
package user

import (
    "encore.dev/storage/sqldb"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	
	"encore.app/user/ent"
)

var userDB = sqldb.NewDatabase("user", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

//encore:service
type Service struct{
    ent *ent.Client
}

func initService() (*Service, error) {
    driver := entsql.OpenDB(dialect.Postgres, userDB.Stdlib())
    entClient := ent.NewClient(ent.Driver(driver))
    return &Service{ent: entClient}, nil
}
```

Now ent is fully wired up to the Encore database, and can be used from the service struct in any API endpoint.

## Using Atlas for database migrations

Finally, we'll set up Atlas to generate database migrations for the ent schema.

First, make sure you [have Atlas installed](https://atlasgo.io/getting-started).

Then, create the file `user/atlas.hcl` containing the following:

```
-- user/atlas.hcl --
env "local" {
  src = "ent://ent/schema"

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

This tells Atlas to generate migrations for the ent schema, and to output them to the `migrations` directory.

Atlas works by comparing the desired ent schema with the current database schema, and generating a migration
to bring the database schema in line with the ent schema. This relies on a so-called "shadow database",
which is an empty database that Atlas uses to compare the ent schema against.

Fortunately for us, Encore has built-in support for shadow databases.

Create the file `user/scripts/generate-migration` containing the following:

```
-- user/scripts/generate-migration --
#!/bin/bash
set -eu
DB_NAME=user
MIGRATION_NAME=${1:-}

# Reset the shadow database
encore db reset --shadow $DB_NAME

# ent executes Go code without initializing Encore when generating migrations,
# so configure the Encore runtime to be aware that this is expected.
export ENCORERUNTIME_NOPANIC=1

# Generate the migration
atlas migrate diff $MIGRATION_NAME --env local --dev-url "$(encore db conn-uri --shadow $DB_NAME)&search_path=public"
```

Finally, make the script executable, and generate our first migration:

```shell
$ chmod +x user/scripts/generate-migration
$ cd user && ./scripts/generate-migration init
```

You should see a new migration file being added to the `user/migrations` directory,
containing the schema changes to create the ent models.

You can now run the service with `encore run`, and everything should be ready to go!
