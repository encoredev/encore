---
seotitle: How to use ORMs like ent for database migrations
seodesc: See how you can use an ORM like ent, or Atlas, to handle your database migrations for your backend application.
title: Use the ent ORM for migrations
---

Encore has all the tools needed to support ORMs and migration frameworks out-of-the-box through
[named databases](/docs/how-to/share-db-between-services) and 
[migration files](/docs/develop/databases#defining-a-database-schema). Writing plain SQL might
not work for your use case, or you may not want to use SQL in the first place. 

ORMs like [ent](https://entgo.io/) or migration frameworks like [Atlas](https://atlasgo.io/) can
be used with Encore by integrating their logic with a system's database. Encore is not restrictive,
it uses plain SQL migration files for its migrations. 

- If your ORM of choice can connect to any database using a [standard SQL driver](https://github.com/lib/pq), then it can be used with Encore using `sqldb.Named()`.
- If your migration framework can generate SQL migration files without any modifications, then it can be used with Encore.

Let's take a look at how you can integrate ent with Encore.

## Add ent schemas to a service
[Install ent](https://entgo.io/docs/tutorial-setup#installation), then initialize your first
schema in the system where you want to use it. For example, if you had the following app structure.

```
/my-app
├── encore.app
└── usr
    ├── org          // org service
    └── user         // user service
```

You can then use this command to generate a user schema along with the ent directory that will contain
that schema and all future generated files:

```
go run entgo.io/ent/cmd/ent init --target usr/ent/schema User
```

The `--target` option sets the schema directory within your Encore system. Each system
should contain its own models and schemas, and its own migration files. Like you would when using
plain SQL.

Add the fields and edges for your new model in the generated file under `usr/ent/schema/user.go`,
then use this command to have ent generate all the files it needs to do its job:

```
go run -mod=mod entgo.io/ent/cmd/ent generate --feature sql/versioned-migration ./usr/ent/schema
```

This generates the client files, as-well-as the logic for generating versioned migrations in SQL. Run
this command again whenever you change the schemas.

### Integrating with a new system
When adding End support in a new Encore system, there are a few steps that need to be completed to make sure the
database exists and the migrations can be created.

First, create the `migrations` directory in the `usr` system and add an empty migration named `1_init.up.sql`. This
migration is necessary for Encore to pick up the system as a [database](/docs/develop/databases) system. Run this
command to have Encore build the application and create the database:

```shell
$ encore run
```

With the database created, you are ready to continue with the guide. You may delete the `1_init` migration or leave
it there, the next steps will work whether it is there or not.

## Connect ent to the system's database
When it generates all its files, ent generates a client interface to connect the ORM to the actual
database through a standard driver. We write something like this to connect the driver with Encore's generated
database, which is very similar to how we'd [connect to an external database](/docs/how-to/connect-existing-db).

**`usr/connectdb.go`**

```go
package usr

import (
	"encore.dev/storage/sqldb"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"go4.org/syncutil"

	"encore.app/usr/ent"
)

var usrDB = sqldb.Named("usr")

// Get returns an ent client connected to this service's database.
func Get() (*ent.Client, error) {
	// Attempt to setup the database client connection if it hasn't
	// already been successfully setup.
	err := once.Do(func() error {
		client = setup()
		return nil
	})
	return client, err
}

var (
	// once is like sync.Once except it re-arms itself on failure
	once syncutil.Once

	// client is the successfully created database client connection,
	// or nil when no such client has been setup yet.
	client *ent.Client
)

// setup sets up a database client connection by opening an ent driver using the
// named database `*sql.DB` pointer and creating a client from that driver.
func setup() *ent.Client {
	drv := entsql.OpenDB(dialect.Postgres, usrDB.Stdlib())
	return ent.NewClient(ent.Driver(drv))
}
```

## Handling migrations
Encore migrations are created in the `migrations` directory as plain SQL files, we need to have ent do the same
for its own generate migrations. When using versioned migrations, ent generates plain SQL migration files using the 
[Atlas](https://atlasgo.io/) migration engine. 

To generate those migration files within an Encore system, you need to configure ent to connect to that system's
database, and to generate the files in that system's migration directory. The following code shows
an example of generating ordered migration files in your `usr` system:

**`commands/generate_migration.go`**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/template"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/sqltool"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/lib/pq"

	"encore.app/usr/ent"
)

const system = "usr"
const dirPathTemplate = "./%s/migrations"
const migrationFilePathTemplate = "%d_ent_migration"

func openPostgresConnection(connectionString string) *sql.Driver {
	driver, err := sql.Open(dialect.Postgres, connectionString)
	if err != nil {
		log.Fatalf("failed to connect to the database. %s", err)
		return nil
	}
	return driver
}

// The migration count is either 1 is the files couldn't be read, or the number of files
// + 1 if we can read them. This makes sure the migration file's index is always incremented.
func createMigrationName() string {
	count := 1
	dirPath := fmt.Sprintf(dirPathTemplate, system)

	files, err := os.ReadDir(dirPath)
	if err != nil {
		log.Printf("failed to list files in the migrations directory, will generate the count as 1. %s", err)
	} else {
		count = len(files) + 1
	}

	return fmt.Sprintf(migrationFilePathTemplate, count)
}

func createMigrateDir() *sqltool.GolangMigrateDir {
	// Create a local migration directory able to understand golang-migrate migration files for replay.
	dirPath := fmt.Sprintf(dirPathTemplate, system)

	dir, err := sqltool.NewGolangMigrateDir(dirPath)
	if err != nil {
		log.Fatalf("failed creating atlas migration directory: %v", err)
		return nil
	}

	return dir
}

func main() {
	ctx := context.Background()
	connectionString := os.Args[1]
	driver := openPostgresConnection(connectionString)
	migrationName := createMigrationName()
	migrateDir := createMigrateDir()

	// Create a formatter for the migration files. This will make sure they generate
	// with a name Encore can parse and valid SQL content. This will only generate the
	// up migrations.
	formatter, err := migrate.NewTemplateFormatter(
		template.Must(template.New("name").Parse("{{ .Name }}.up.sql")),
		template.Must(template.New("name").Parse(`{{range .Changes}}{{print .Cmd}};{{ println }}{{end}}`)),
	)
	if err != nil {
		log.Fatalf("failed creating an atlas formatter: %v", err)
	}

	// Create a client for the migration using the SQL driver connected to the system's database
	versionedClient := ent.NewClient(ent.Driver(driver))

	// Write the migration diff without a checksum file
	// (Encore expects only SQL files in the migration directory)
	opts := []schema.MigrateOption{
		schema.WithDir(migrateDir),
		schema.DisableChecksum(),
		schema.WithFormatter(formatter),
	}

	// Generate migrations using Atlas.
	err = versionedClient.Schema.NamedDiff(ctx, migrationName, opts...)
	if err != nil {
		log.Fatalf("failed generating migration file: %v", err)
	}
}
```

Execute the following command to have this script generate your first migration file:

```
go run -mod=mod ./commands/generate_migration.go $(encore db conn-uri usr)
```

Finally, run `encore run` to generate your system and apply the migrations. The next execution
of the migration script will diff against this newly migrated database and only generate SQL
for what actually changed.

<Callout type="info">

Running the migration script multiple times without first running `encore run`
will cause multiple migrations to be created with the same content. Make sure to apply
all previous migrations before generating a new one.

</Callout>

That's it! You can now use the `Get` function from your system to connect your ent client
to the system's database and generate migrations while still using Encore's simple migration
and database management system, like this:

```go
package usr

import (
	"context"

	"encore.dev/beta/errs"
)

type GetUserResponse struct {
    ID   int
    Age  int
    Name string
}

//encore:api public path=/users/:id
func GetUser(ctx context.Context, id int) (*GetUserResponse, error) {
	client, err := Get()
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "Database connection is closed",
		}
	}

	user, err := client.User.Get(ctx, id)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "Could not find user",
		}
	}

	return &GetUserResponse{
		ID:   user.ID,
		Name: user.Name,
		Age:  user.Age,
	}, nil
}
```

<Callout type="info">

ent types cannot be used as parameter types or return types in Encore endpoints; they contain values that are not
marshalable. You must use your own struct and mirror the fields you want to send back to your users.

</Callout>
