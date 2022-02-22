---
title: Share SQL databases between services
---

By default, each service in an Encore app has its own database. This approach has many benefits: 
- Which database is used and how it works is abstracted away from other services
- The database is more isolated, making changes to it smaller and safer
- By making the services more independent your application becomes more reliable by being able to more gracefully handle partial outages, such as if your database is temporarily overloaded or offline.

But like everything else in software engineering, there are trade-offs involved, and sometimes it's simpler and more reliable to use a single database that's accessed by multiple services. Encore makes this easy to do.

Each database in Encore is defined within a service. That service's name becomes the name of the database. Other services can then access that database by creating a database reference with `sqldb.Named("dbname")`.

## Example

Let's say you have a simple `todo` service, with only one table:

**`todo/migrations/1_create_table.up.sql`**

```sql
CREATE TABLE todo_item (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    done BOOLEAN NOT NULL DEFAULT FALSE
);
```

You want to create a `report` service that produces various reports for internal business processes, but for simplicity you decide it makes sense to directly access the `todo` database. All that's needed is to define the `todoDB` variable like so:

**`report/report.go`**

```go
package report

import (
	"context"

	"encore.dev/storage/sqldb"
)

// todoDB connects to the "todo" service's database.
var todoDB = sqldb.Named("todo")

type ReportResponse struct {
    Total int
}

// CountCompletedTodos generates a report with the number of completed todo items.
//encore:api method=GET path=/report/todo
func CountCompletedTodos(ctx context.Context) (*ReportResponse, error) {
    var report ReportResponse
    err := todoDB.QueryRow(ctx,`
        SELECT COUNT(*)
        FROM todo_item
        WHERE completed = TRUE
    `).Scan(&report.Total)
    return &report, err
}
```

With that, Encore understands that the `report` service depends on the `todo` service's database, and orchestrates the necessary connections to make that happen. And like everything else with Encore, it works exactly the same regardless of where it's running: for local development as well as in the cloud.
