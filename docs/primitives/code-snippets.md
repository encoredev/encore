---
seotitle: Code snippets for using the Infrastructure SDK's building blocks in your backend application
seodesc: Learn how to build cloud-agnostic backend applications using Encore's Infrastructure SDK.
title: Code snippets
subtitle: Shortcuts for building with Encore
---

When you're familiar with how Encore works, you can simplify your development workflow by copy-pasting these examples. If you're looking for details on how Encore works, please refer to the relevant docs section.

## APIs

### Defining APIs

```go
package hello // service name

//encore:api public
func Ping(ctx context.Context, params *PingParams) (*PingResponse, error) {
    msg := fmt.Sprintf("Hello, %s!", params.Name)
    return &PingResponse{Message: msg}, nil
}
```

### Defining Request and Response schemas

```go
// PingParams is the request data for the Ping endpoint.
type PingParams struct {
    Name string
}

// PingResponse is the response data for the Ping endpoint.
type PingResponse struct {
    Message string
}
```

### Calling APIs

```go
import "encore.app/hello" // import service

//encore:api public
func MyOtherAPI(ctx context.Context) error {
    resp, err := hello.Ping(ctx, &hello.PingParams{Name: "World"})
    if err == nil {
        log.Println(resp.Message) // "Hello, World!"
    }
    return err
}
```

**Hint:** Import the service package and call the API endpoint using a regular function call.

### Receive Webhooks

```go
import "net/http"

// Webhook receives incoming webhooks from Some Service That Sends Webhooks.
//encore:api public raw
func Webhook(w http.ResponseWriter, req *http.Request) {
    // ... operate on the raw HTTP request ...
}
```

**Hint:** Like any other API endpoint, this will be exposed at:<br/>
`https://<env>-<app-id>.encr.app/service.Webhook`

## Databases

### Creating a SQL database

To create a database, import `encore.dev/storage/sqldb` and call `sqldb.NewDatabase`, assigning the result to a package-level variable.
`sqldb.DatabaseConfig` specifies the directory containing the database migration files, which is how you define the database schema.

```
-- todo/db.go --
package todo

// Create the todo database and assign it to the "tododb" variable
var tododb = sqldb.NewDatabase("todo", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// Then, query the database using db.QueryRow, db.Exec, etc.
-- todo/migrations/1_create_table.up.sql --
CREATE TABLE todo_item (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  done BOOLEAN NOT NULL DEFAULT false
  -- etc...
);
```

### Inserting data into a database

One way of inserting data is with a helper function that uses the package function `sqldb.Exec`:

```go
import "encore.dev/storage/sqldb"

// insert inserts a todo item into the database.
func insert(ctx context.Context, id, title string, done bool) error {
	_, err := tododb.Exec(ctx, `
		INSERT INTO todo_item (id, title, done)
		VALUES ($1, $2, $3)
	`, id, title, done)
	return err
}
```

### Querying a database

To read a single todo item in the example schema above, we can use `sqldb.QueryRow`:

```go
import "encore.dev/storage/sqldb"

var item struct {
    ID int64
    Title string
    Done bool
}
err := tododb.QueryRow(ctx, `
    SELECT id, title, done
    FROM todo_item
    LIMIT 1
`).Scan(&item.ID, &item.Title, &item.Done)
```

**Hint:** If `sqldb.QueryRow` does not find a matching row, it reports an error that can be checked against
by importing the standard library `errors` package and calling `errors.Is(err, sqldb.ErrNoRows)`.

## Defining a Cron Job

```go
import "encore.dev/cron"

var _ = cron.NewJob("welcome-email", cron.JobConfig{
	Title:    "Send welcome emails",
	Every:    2 * cron.Hour,
	Endpoint: SendWelcomeEmail,
})

//encore:api private
func SendWelcomeEmail(ctx context.Context) error {
	// ...
	return nil
}
```
**Hint:** Cron Jobs do not run in your local development environment.

## PubSub

### Creating a PubSub topic

```go
import "encore.dev/pubsub"

type SignupEvent struct { UserID int }
var Signups = pubsub.NewTopic[*SignupEvent]("signups", pubsub.TopicConfig {
    DeliveryGuarantee: pubsub.AtLeastOnce,
})
```

**Hint:** Topics are declared as package level variables and cannot be created inside functions. Regardless of where you create a topic, it can be published and subscribed to from any service.

### Publishing an Event (Pub)

```go
if _, err := Signups.Publish(ctx, &SignupEvent{UserID: id}); err != nil {
    return err
}

if err := tx.Commit(); err != nil {
    return err
}
```

**Hint:** If you want to publish to the topic from another service, import the topic package variable (`Signups` in this example) and call publish on it from there.

### Subscribing to Events (Sub)

Create a Subscription as a package level variable by calling `pubsub.NewSubscription`.

```go
var _ = pubsub.NewSubscription(
    user.Signups, "send-welcome-email",
    pubsub.SubscriptionConfig[*SignupEvent] {
        Handler: SendWelcomeEmail,
    },
)
func SendWelcomeEmail(ctx context.Context, event *SignupEvent) error {
    ... send email ...
    return nil
}
```

## Defining a Cache cluster

```go
import "encore.dev/storage/cache"

var MyCacheCluster = cache.NewCluster("my-cache-cluster", cache.ClusterConfig{
    // EvictionPolicy tells Redis how to evict keys when the cache reaches
    // its memory limit. For typical cache use cases, cache.AllKeysLRU is a good default.
    EvictionPolicy: cache.AllKeysLRU,
})
```

## Secrets

### Defining Secrets

```go
var secrets struct {
    GitHubAPIToken string   // personal access token for deployments
    SomeOtherSecret string  // some other secret
}
```

**Hint:** The variable must be an unexported struct named `secrets`, and all the fields must be of type `string`.

### Setting secret values

```shell
$ encore secret set --type <types...> <secret-name>
```

**Hint:** `<types>` defines which environment types the secret value applies to. Use a comma-separated list of `production`, `development`, `preview`, and `local`. For each Secret, there can only be one secret value for each environment type.

### Using secrets

```go
func callGitHub(ctx context.Context) {
    req, _ := http.NewRequestWithContext(ctx, "GET", "https:///api.github.com/user", nil)
    req.Header.Add("Authorization", "token " + secrets.GitHubAPIToken)
    resp, err := http.DefaultClient.Do(req)
    // ... handle err and resp
}
```

**Hint:** Secret keys are globally unique for your whole application; if multiple services use the same secret name they both receive the same secret value at runtime.