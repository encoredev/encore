---
seotitle: How to deploy your Encore application with an existing AWS RDS instance
seodesc: Learn how to easily import your existing AWS RDS instance and connect your Encore application to it.
title: Import an existing AWS RDS instance
subtitle: Using your pre-existing database instead of provisioning a new one
lang: platform
---

# Overview

When deploying applications to your own cloud, Encore Cloud can provision all necessary infrastructure—including database instances. However, if you already have an AWS RDS instance, you can connect your Encore application directly to this existing database.

## Benefits

Using an existing AWS RDS instance allows you to:
- Maintain data continuity with your existing systems
- Preserve specific database configurations
- Utilize familiar database setups without migration

## Importing an AWS RDS instance

Follow these steps to import your existing AWS RDS instance:

1. Navigate to **Create Environment** in the [Encore Cloud dashboard](https://app.encore.cloud)
2. Select the AWS cloud provider
3. Pick the `AWS Region` your database resides in
3. Choose **Import Existing RDS Instance**
4. Specify your database's `RDS Instance Name`
5. Click the `Resolve` button to validate the instance

Once validated, you can create the environment. When you deploy to this environment, Encore Cloud will automatically connect your application to your imported AWS RDS instance rather than provisioning a new database.

## Mapping existing databases to your Encore app
To access an existing database in your Encore application, you need to specify the name of the existing database when you declare the database in your app. For example, if you have an existing database called `mydb` you can create a reference to it like so:

```typescript
const db = new SQLDatabase("mydb");
```

```go
sqldb.NewDatabase("mydb", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})
```

## Applying migrations to existing databases
Encore uses a table called `schema_migrations` in the public namespace to keep track of which migrations have been applied. If you import an existing database without that table, Encore will create it for you and apply your migrations in order. If the table already exists, Encore expects it to contain exactly two columns:

```
version bigint
dirty boolean
```

If the table exists but has a different schema, you will not be able to import it with Encore at this time. If the table exists with an existing entry, Encore will apply all higher versions in your `migrations` directory to the database.
