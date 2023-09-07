---
title: Add a SQL database
---

In the previous step you deployed your app to production. Now you'll learn how to add a SQL database to store some data.

<Callout type="important">

To locally run Encore apps with databases, you need to have [Docker](https://www.docker.com) installed and running.

</Callout>

## 1. Add a new API endpoint

We'll add a new API endpoint, `hello.There` that takes a name as input
and responds with a personalized greeting. The database will store
the names of people and how many times you've met.

Create a new file named `hello/there.go` with the following contents:

```go
package hello

import (
    "context"
    "fmt"
)

type ThereParams struct {
    Name string
}

type ThereResponse struct {
    Message string
}

// There responds with a personalized greeting.
//encore:api public
func There(ctx context.Context, params *ThereParams) (*ThereResponse, error) {
    greeting, err := generateGreeting(ctx, params.Name)
    if err != nil {
        return nil, err
    }
    return &ThereResponse{Message: greeting}, nil
}

// generateGreeting generates a personalized greeting for name.
func generateGreeting(ctx context.Context, name string) (string, error) {
    msg := fmt.Sprintf("Hello, %s!", name)
    return msg, nil
}
```

Run your app with `encore run` and then open the local development dashboard by visiting [http://localhost:9400](http://localhost:9400) in your browser.

Navigate to the API Documentation by clicking **API** in the top left.
There you'll see your new `hello.There` endpoint.
Make an API Call by clicking on `Call` and enter `"Jane"` as your name.

You'll immediately see a personalized response:
```json
{"Message": "Hello, Jane!"}
```

It works! Now it's time to add a database to track how many times you've met.

## 2. Create a database table

Inside your `hello` service, add a new directory named `migrations`.

Inside this folder, create a new file named `1_create_table.up.sql`, with the
following contents:

```sql
CREATE TABLE people (
  name TEXT PRIMARY KEY,
  count INT NOT NULL
);
```

This will create a database table named `people`, with `name` as the primary key
along with `count` tracking how many times you've met.

## Query the database

Now it's time to use the database in your API.
Go back to `hello/there.go` and modify the `import` declaration at the top
to include `encore.dev/storage/sqldb`:

```go
import (
  "context"
  "fmt"

  "encore.dev/storage/sqldb"
)
```

Then, modify the `generateGreeting` function at the bottom to look like this:

```go
func generateGreeting(ctx context.Context, name string) (string, error) {
  // Make an "UPSERT" (insert or update) query,
  // inserting a row if it doesn't already exist
  // and otherwise updating it by incrementing count.
  var count int
  err := sqldb.QueryRow(ctx, `
    INSERT INTO "people" (name, count)
    VALUES ($1, 1)
    ON CONFLICT (name) DO UPDATE
    SET count = people.count + 1
    RETURNING count
  `, name).Scan(&count)
  if err != nil {
    return "", err
  }
  
  var greeting string
  if count > 1 {
    greeting = fmt.Sprintf("Hey again %s! We've met %d time(s) before.", name, count-1)
  } else {
    greeting = fmt.Sprintf("Nice to meet you, %s!", name)
  }
  return greeting, nil
}
```

This will query the database, and insert a row for the given name with `count = 1`.
If the row already exists, it instead increments `count`.
Finally it returns the stored `count` value.

We scan this value into the `count` variable and then generate a personalized
greeting based on the count.

Try it out by running `encore run`. Either open the API Documentation and try again,
or run in a separate terminal:

```shell
$ curl http://localhost:4000/hello.There -d '{"Name": "John"}'
{"Message": "Nice to meet you, John!"}
$ curl http://localhost:4000/hello.There -d '{"Name": "John"}'
{"Message": "Hey again John! We've met 1 time(s) before."}
$ curl http://localhost:4000/hello.There -d '{"Name": "John"}'
{"Message": "Hey again John! We've met 2 time(s) before."}
```

Wonderful, it works!

## 3. Deploy to production

To deploy this to production, simply push this code to Encore:

```shell
$ git add -A
$ git commit -m "Add hello.There endpoint with a personalized greeting"
$ git push encore
```
This triggers a build and deploy to the cloud.

Head over to your app's production dashboard by opening [app.encore.dev](https://app.encore.dev) in your browser.
Once the deploy completes, go to the API page and make an API call
to your new `hello.There` endpoint.

You should see the same thing as before:
```json
{"Message": "Nice to meet you, John!"}
{"Message": "Hey again John! We've met 1 time(s) before."}
```

Excellent! You now have a database up and running in production,
and an API hooked up to it.
