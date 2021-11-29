---
title: Share SQL databases between services
---

By default, each service in an Encore app has it's own database, running in its own isolated environment. This approach has huge benefits: load balancing between services and making failure of the whole app almost absolutely impossible to name a few.

But in some cases you might want to get access to a database, that belongs to another service. Since the release of v0.17 we can now reference databases by name with `sqldb.Named("name")`, enabling multiple services to reference a single database.

## Example

Let's say you have a simple `todo` service, with the only one table:

**`todo/migrations/1_create_table.up.sql`**

```sql
CREATE TABLE todo_item (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    done BOOLEAN NOT NULL DEFAULT FALSE
);
```

If you decide to create a `report` service, generating reports in different formats or whatnot.
Let's say you would like to generate a report, showing the amount of completed todo items.
You could do it the following way:

**`report/report.go`**

```go
package report

import (
	"context"

	"encore.dev/storage/sqldb"
)

type ReportResponse struct {
    Total int
}

//Count completed todo items
//encore:api method=GET path=/report/todo
func CountCompletedTodo(ctx context.Context) (*ReportResponse, error){
    var report ReportResponse
    err := sqldb.Named("todo").
			QueryRow(ctx,`SELECT COUNT(*) FROM todo_item WHERE completed = TRUE`).
			Scan(&report.Total)
    return &report, err
}
```
