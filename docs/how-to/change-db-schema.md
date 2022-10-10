---
title: Change SQL database schema
---

Encore database schemas are changed over time using *migration files*.

Each migration file has a sequence number, and migration files are run
in sequence when deploying. Encore tracks which migrations have already run
and only runs new ones.

To change your database schema, add a new migration file using the next
available migration number.

For example, if you have two migration files already,
the next migration file should be named `3_something.up.sql` where
`something` is a short description of what the migration does.

<Callout type="warning">

Database migrations are applied before the application is restarted
with the new code. Always make sure the old application code works with
the new database schema, so that things don't break while your new code
is being rolled out.

</Callout>

## Example

Let's say you have a single migration file that creates a `todo_item` table:

**`todo/migrations/1_create_table.up.sql`**
```sql
CREATE TABLE todo_item (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    done BOOLEAN NOT NULL
);
```

And now you want to add a `created` column to track when each todo was created.
Add a new file:

**`todo/migrations/2_add_created_col.up.sql`**
```sql
ALTER TABLE todo_item ADD created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW();
```

The next deploy Encore will notice the new migration file and run it, adding
a new column.