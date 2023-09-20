---
seotitle: How to insert test data in a database
seodesc: Learn how to populate your database with test data using Go and Encore, making testing your backend application much simpler.
title: Insert test data in a database
---

When you're developing or testing, it's often useful to seed databases with test data.
This can be done is several ways depending on your use case.

## Using go:embed

A straightforward way to insert test data is to conditionally insert it on startup using `go:embed` in combination with Encore's [metadata API](/docs/develop/metadata) control in which environments the data gets inserted. E.g. only in your local environment.

### Example

Create a file with your test data named `fixtures.sql`.
Then, for the service where you want to insert test data, add the following to its `.go` file in order to run on startup.

```
import (
    _ "embed"
    "log"

    "encore.dev"
)

//go:embed fixtures.sql
var fixtures string

func init() {
    if encore.Meta().Environment.Cloud == encore.CloudLocal {
        if _, err := sqldb.Exec(context.Background(), fixtures); err != nil {
            log.Fatalln("unable to add fixtures:", err)
        }
    }
}
```

Not included in the above example is preventing adding duplicate data. This is straightforward to do by making the fixtures idempotent, or by tracking it with a database table.

## Populating databases in Preview Environments

In some cases, it can be useful to populate new Preview Environments with test data to simplify testing. 

The best way to do this depends a bit on your use case, but a common way to do this is by using Encore's [webooks](/docs/deploy/webhooks) functionality, which provides notifications for when a deployment is completed and includes information about the environment in question.