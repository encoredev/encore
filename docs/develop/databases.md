---
title: Using SQL databases
subtitle: Provisioning, migrating, querying
---

Encore treats SQL databases as logical resources.
This means using a database only requires you to [define the schema](#defining-a-database-schema)
and then start using it. Encore takes care of provisioning the database, running
new schema migrations during deploys, and connecting to it.

Encore's SQL databases are **PostgreSQL** databases.

## Defining a database schema

Database schemas are defined by creating *migration files* in a directory named `migrations`
within an Encore service package. Each migration file is named `<number>_<name>.up.sql`, where
`<number>` is a sequence number for ordering the migrations and `<name>` is a
descriptive name of what the migration does.

Each migration runs in order, and expresses the change in the database schema
from the previous migration.

The first migration usually defines the initial table structure. For example,
a `todo` service might start out by creating `todo/migrations/1_create_table.up.sql` with
the following contents:

```sql
CREATE TABLE todo_item (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    done BOOLEAN NOT NULL DEFAULT false
);
```

## Querying databases

Once you have defined a database schema, you can easily query it.
Simply import `encore.dev/storage/sqldb` in your service package (or any sub-packages within the service),
and start using the package functions. The interface is similar to that of the Go standard library's
`database/sql` package.

For example, to read a single todo item using the example schema above:

```go
var item struct {
    ID int64
    Title string
    Done bool
}
err := sqldb.QueryRow(ctx, `
    SELECT id, title, done
    FROM todo_item
    LIMIT 1
`).Scan(&item.ID, &item.Title, &item.Done)
```

### Connecting to databases

It's often useful to be able to connect to the database from outside the backend application.
For example for scripts, ad-hoc querying or dumping data for analysis.

The Encore CLI comes with built in support for this:

* Use `encore db shell [--env=<name>] <service-name>` to open a [psql](https://www.postgresql.org/docs/current/app-psql.html)
  shell to the database for `<service-name>` in the given environment.
  Leaving out `--env` defaults to the local development environment.

* Use `encore db proxy [--env=<name>]` to create a local proxy that forwards any incoming connection
  to the database in the given environment.
  Leaving out `--env` defaults to the local development environment.

See `encore help db` for more information on database management commands.

## Provisioning databases

Encore automatically provisions databases in a suitable way depending on the environment.
When running locally, Encore creates a database cluster using docker.
In the cloud, how the database is provisioned depends on the type of [Encore Environment](/docs/concepts/environments):

- In `production` environments, the database is provisioned through the Managed SQL Database
  service offered by the chosen cloud provider.
- In `development` environments, the database is provisioned as a Kubernetes deployment
  with a persistent disk attached.

Encore automatically provisions databases to match what your application requires.
Simply define a database schema ([see above](#defining-a-database-schema)) and Encore
will provision the database at the start of the next deploy.
