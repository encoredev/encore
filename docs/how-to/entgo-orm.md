---
title: Use the ent ORM for migrations
---

Encore has all the tools needed to support ORMs and migration frameworks out of the box through
[named databases](/docs/how-to/share-db-between-services) and 
[migration files](/docs/develop/databases#defining-a-database-schema). Writing plain SQL might
not work for your use case, or you may not want to use SQL in the first place. 

ORMs like [ent](https://entgo.io/) or migration frameworks like [Atlas](https://atlasgo.io/) can
be used with Encore by integrating their logic with a system's database. Encore is not restrictive,
it uses plain SQL migration files for its migrations. 

> If your ORM of choice can connect to any database using a 
> [standard SQL driver](https://github.com/lib/pq), then it can be used with encore 
> using `sqldb.Named()`
> 
> If your migration framework can generate SQL migration files without any modifications.

Let's take a look at how you may integrate ent with encore.

## Add ent schemas to a service
[Install ent](https://entgo.io/docs/getting-started#installation), then initialize your first
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

The `--target` option sets the schema directory within your encore system. Each system
should contain its own models and schemas, and its own migration files. Like you would when using
plain SQL.

Add the fields and edges for your new model in the generated file under `usr/ent/schema/user.go`,
then use this command to have ent generate all the files it needs to do its job:

```
go run -mod=mod entgo.io/ent/cmd/ent generate --feature sql/versioned-migration ./usr/ent/schema
```

This generates the client files, as-well-as the logic for generating versioned migrations in SQL. Run
this command again whenever you change the schemas.

## Connect ent to the system's database
When it generates all its files, ent generates a client interface to connect the ORM to the actual
database through a standard driver. We write something like this to connect the driver with encore's generated
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
	"io/ioutil"
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

func main() {
	ctx := context.Background()

	connectionString := os.Args[1]

	// Open an SQL connection to the system's postgres database.
	driver, err := sql.Open(dialect.Postgres, connectionString)
	if err != nil {
		log.Fatalf("failed to connect to the database. %s", err)
	}

	// Get the migration directory and read the files
	dirPath := fmt.Sprintf("./%s/migrations", system)
	files, err := ioutil.ReadDir(dirPath)
	
	// The migration count is either 1 is the files couldn't be read, or the number of files
	// + 1 if we can read them. This makes sure the migration file's index is always incremented.
	migrationCount := 1
	if err != nil {
		log.Printf("failed to list files in the migrations directory, will generate the count as 1. %s", err)
	} else {
		migrationCount = len(files) + 1
	}

	// Create a local migration directory able to understand golang-migrate migration files for replay.
	dir, err := sqltool.NewGolangMigrateDir(dirPath)
	if err != nil {
		log.Fatalf("failed creating atlas migration directory: %v", err)
	}

	// Create a formatter for the migration files. This will make sure they generate
	// with a name encore can parse and valid SQL content. This will only generate the
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
	// (Encore expects only SQL files in the migrations directory)
	opts := []schema.MigrateOption{
		schema.WithDir(dir),
		schema.DisableChecksum(),
		schema.WithFormatter(formatter),
	}

	// Generate migrations using Atlas.
	err = versionedClient.Schema.NamedDiff(ctx, fmt.Sprintf("%d_ent_migration", migrationCount), opts...)
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

> Note that running the migration script multiple times without first running `encore run`
> will cause multiple migrations to be created with the same content. Make sure to apply
> all previous migrations before generating a new one.

That's it! You can now use the `Get` function from your system to connect your ent client
to the system's database and generate migrations while still using Encore's simple migration
and database management system.
