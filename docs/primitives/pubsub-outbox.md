---
seotitle: Using a transactional Pub/Sub outbox
seodesc: Learn how you can use a transactional outbox with Pub/Sub to guarantee consistency between your database and Pub/Sub subscribers
title: Transactional Pub/Sub outbox
subtitle: Guarantee consistency between your database and Pub/Sub subscribers
---

One of the hardest parts of building an event-driven application is ensuring consistency between services.
A common pattern is for each service to have its own database and use Pub/Sub to notify other systems of business events.
Inevitably this leads to inconsistencies since the Pub/Sub publishing is not transactional with the database writes.

While there are several approaches to solving this, it's important the solution doesn't add too much complexity
to what is often an already complex architecture. Perhaps the best solution in this regard is the [transactional outbox pattern](https://softwaremill.com/microservices-101/).

Encore provides support for the transactional outbox pattern in the [x.encore.dev/infra/pubsub/outbox](https://pkg.go.dev/x.encore.dev/infra/pubsub/outbox) package.

The transactional outbox works by binding a Pub/Sub topic to a database transaction, translating all calls to `topic.Publish`
into inserting a database row in an `outbox` table. If/when the transaction later commits, the messages are picked up by
a [Relay](https://pkg.go.dev/x.encore.dev/infra/pubsub/outbox#Relay) that polls the `outbox` table and publishes the
messages to the actual Pub/Sub topic.

## Publishing messages to the outbox

To publish messages to the outbox, a topic must first be bound to the outbox. This is done using
[Pub/Sub topic references](/docs/primitives/pubsub#using-topic-references) which allows you to retain complete
type safety and the same interface as regular Pub/Sub topics, allowing existing code to continue to work without changes.

<Callout type="info">

In regular (non-outbox) usage the message id returned by `topic.Publish` is the same as the message id the subscriber
receives when processing the message. With the outbox, this message id is not available until the transaction commits,
so `topic.Publish` returns an id referencing the outbox row instead.

</Callout>


The topic binding supports pluggable storage backends, enabling use of the outbox pattern with any
transactional storage backend. Implementation are provided out-of-the-box for use with Encore's
`encore.dev/storage/sqldb` package, as well as the standard library `database/sql` and `github.com/jackc/pgx/v5` drivers,
but it's easy to write your own for other use cases.
See the [Go package reference](https://pkg.go.dev/x.encore.dev/infra/pubsub/outbox#PersistFunc) for more information.

For example, to use a transactional outbox to notify subscribers when a user is created:

```go
-- outbox.go --
// Create a SignupsTopic somehow.
var SignupsTopic = pubsub.NewTopic[*SignupEvent](/* ... */)

// Create a topic ref with publisher permissions.
ref := pubsub.TopicRef[pubsub.Publisher[*SignupEvent]](SignupsTopic)

// Bind it to the transactional outbox
import "x.encore.dev/infra/pubsub/outbox"
var tx *sqldb.Tx // somehow get a transaction
ref = outbox.Bind(ref, outbox.TxPersister(tx))

// Calls to ref.Publish() will now insert a row in the outbox table.

-- db_migration.sql --
-- The database used must contain the below database table:
-- See https://pkg.go.dev/x.encore.dev/infra/pubsub/outbox#SQLDBStore
CREATE TABLE outbox (
    id BIGSERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    data JSONB NOT NULL,
    inserted_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX outbox_topic_idx ON outbox (topic, id);
```

Once the transaction commits any published messages via `ref` above will be stored in the `outbox` table.

## Consuming messages from the outbox

Once committed, the messages are ready to be picked up and published to the actual Pub/Sub topic.

That is done via the [Relay](https://pkg.go.dev/x.encore.dev/infra/pubsub/outbox#Relay).
The relay continuously polls the `outbox` table and publishes any new messages to the actual Pub/Sub topic.

The relay supports pluggable storage backends, enabling use of the outbox pattern with any
transactional storage backend. An implementation is provided out-of-the-box that uses Encore's built-in
[SQL database support](https://pkg.go.dev/x.encore.dev/infra/pubsub/outbox#SQLDBStore),
but it's easy to write your own for other databases.

The topics to poll must be registered with the relay, typically during service initialization. For example:

```go
-- user/service.go --
package user

import (
	"context"
	
    "encore.dev/pubsub"
    "encore.dev/storage/sqldb"
    "x.encore.dev/infra/pubsub/outbox"
)

type Service struct {
	signupsRef pubsub.Publisher[*SignupEvent]
}

// db is the database the outbox table is stored in
var db = sqldb.NewDatabase(...)

// Create the SignupsTopic somehow.
var SignupsTopic = pubsub.NewTopic[*SignupEvent](/* ... */)

func initService() (*Service, error) {
    // Initialize the relay to poll from our database.
	relay := outbox.NewRelay(outbox.SQLDBStore(db))
	
	// Register the SignupsTopic to be polled.
    signupsRef := pubsub.TopicRef[pubsub.Publisher[*SignupEvent]](SignupsTopic)
	outbox.RegisterTopic(relay, signupsRef)
	
	// Start polling.
	go relay.PollForMessage(context.Background(), -1)
	
	return &Service{signupsRef: signupsRef}, nil
}
```
