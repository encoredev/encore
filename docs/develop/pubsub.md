---
title: PubSub
subtitle: Decoupling applications for better reliability
---

Building software using publishers & subscribers (PubSub) allows you to build systems which communicate with each other
by broadcasting events asynchronously. Events are a great way to decouple applications for better reliability by reducing
the blast radius of faulty components and bottlenecks. They increase the speed of response to the user, and they also
help reduce cognitive overhead for developers by inverting the dependencies between services.

# The benefits of PubSub

Let's take an example of an API which registers new users. Upon registration, we want to send a welcome email to the user
and create a record of the signup in our analytics system.


### An API only approach

Using API calls between services, we might design a system which looks like this when the user registers:

<div className="grid grid-cols-3 mobile:grid-cols-1 grid-flow-row">


<img src="/assets/docs/pubsub-rpc-example.png" className="noshadow w-100" />

<div className="col-span-2">

1. The `user` service starts a database transaction and records the user in its database.
2. The `user` service makes a call to the `email` service to send a welcome email.
3. The `email` service then calls an email provider to actually send the email.
4. Upon success, the `email` service replies to the `user` service that the request was processed.
5. The `user` service then calls the `analytics` service to record the signup.
6. The `analytics` service the writes to the data warehouse to record the information.
7. The `analytics` service then replies to the `user` service that the request was processed.
8. The `user` service commits the database transaction.
9. The `user` service then can reply to the user to say the registration was successful.

</div>

</div>

Notice how we have to wait for everything to complete before we can reply to the user to tell then we've registered them.
This means that if our email provider takes 3 seconds to send the email, we've now taken 3 seconds to respond to the user,
when in reality once the user was written to the database, we could have responded to the user instantly at that point to
confirm the registration.

Another downside to this approach is if our data warehouse is currently broken and reporting errors, our system will also
report errors whenever anybody tries to signup! Given analytics is purely internal and doesn't impact users, why should
the analytics system being down impact user signup?

### A PubSub approach

A more ideal solution would be if we could decouple the behaviour of emailing the user and recording our analytics, such that
the user service only has to record the user in its own database and let the user know they are registered - without worrying
about the downstream impacts. Thankfully, this is exactly what [PubSub topics](https://pkg.go.dev/encore.dev/pubsub#Topic) allow us to do.

<div className="grid grid-cols-3 mobile:grid-cols-1 grid-flow-row">

<div className="col-span-2">

In this example, when a user registers we:

1. The `user` service starts a database transaction and records the user in its database.
2. Publish a signup event to the `signups` topic.
3. Commit the transaction and reply to the user to say the registration was successful.

At this point the user is free to continue interacting with the application and we've isolated the registration behaviour
from the rest of the application.

In parallel, the `email` and `analytics` services will receive the signup event from the `signups` topic and will then
perform their respective tasks. If either service returns an error, the event will automatically be backed off and retried
until the service is able to process the event successfully, or reaches the maximum number of attempts and is placed
into the deadletter queue (DLQ).

</div>

<img src="/assets/docs/pubsub-topic-example.png" className="noshadow w-100" />

</div>

<Callout type="info">

Notice how in this version, the processing time of the two other services did not impact the end user and in fact the `user`
service is not even aware of the `email` and `analytics` services. This means that new systems which need to know about
new users signing up can be added to the application, without the need to change the `user` service or impacting its
performance.

</Callout>


# Worked Example

## Creating a Topic

The core of PubSub is the concept of a Topic, which is a named channel that can be used to publish events on. Topics
must be created inside services, but they can be published to from any service and can be subscribed to from any service.

Topics must be given an event type, a unique name and a configuration to define their behaviour. The configuration allows
us to define the delivery guarantee of the topic. Currently, only `pubsub.AtLeastOnce` is supported; however, it must be
defined in the topic configuration to make ensure all developers understand the semantics of the topic.

```go
package user

import "encore.dev/pubsub"

type SignupEvent struct { UserID int }
var Signups = pubsub.NewTopic[*SignupEvent]("signups", pubsub.TopicConfig {
    DeliveryGuarantee: pubsub.AtLeastOnce,
})
```

<Callout type="info">

Topics must be declared as package level variables, and cannot be created inside functions.

</Callout>

### At-least-once delivery

This behaviour for topics, means that for each subscription events will be delivered _at least once_.
This means if the topic believes the event was not processed, it will attempt to deliver the message another time.
As such, all subscription handlers should be [idempotent](https://en.wikipedia.org/wiki/Idempotence#Computer_science_meaning);
this means if the handler is called two or more times, from the view of the outside world, there should be no difference
as if it was only called once.

This can be achieved using the database to track if you have already performed the action which the event needs you to do,
or ensuring that the action being performed is also idempotent in nature.

## Publishing an Event (Pub)

To publish an event, we simply call `Publish` on the Topic with the event

```go
package user

import (
    "encore.dev/storage/sqldb"
    "encore.dev/pubsub"
)

//encore:api public
func Register(ctx context.Context, params *RegistrationParams) error {
    tx, err := sqldb.Begin(ctx) // start a database transaction
    defer tx.Rollback() // rollback the transaction if we don't commit it

    ... process registration ...

    // publish the event
    if _, err := Signups.Publish(ctx, &SignupEvent{UserID: id}); err != nil {
        return err
    }
    // at this point we know the any subscribers will receive the event at some point
    // in the future.

    // then commit the transaction, this way if the publishing of the event fails
    // the user isn't created, however if it succeeds then we can return OK now
    // and let the other processes handle the event asynchronously without risking
    // the user being created without the event being published.
    if err := tx.Commit(); err != nil {
        return err
    }

    return nil
}
```

If we wanted to publish to the topic from another service, then we could just import the `Signups` package variable
and call publish on it from there.

## Subscribing to Events (Sub)

To subscribe to events, we need to create a Subscription as a package level variable by calling the
[`pubsub.NewSubscription`](https://pkg.go.dev/encore.dev/pubsub#NewSubscription) function. Each subscription needs the topic
to subscriber to, a name which is unique for the topic and a configuration object, with at least a
`Handler` function to process the events and a configuration object.

```go
package email

import (
    "encore.dev/pubsub"
    "user"
)

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

A subscription can be in the same service as the topic is declared, or in any other service of your application. Each
subscription to a single topic receives the events independently of any other subscriptions to the same topic. This means
that if one subscription is running very slowly it will grow a backlog up, however other subscriptions will still be
processing events in real time as they are published.

<Callout type="important">

Note: The `SubscriptionConfig` struct fields must all be defined as compiled time constants, and cannot be defined in
terms of function calls. This is to allow Encore to understand the exact requirements for the subscription and provision
the correct infrastructure upon deployment to the cloud.

</Callout>

### Error Handling

If a subscription function returns an error, the event which was being processed will be retried, based on the retry policy
[configured on that subscription](https://pkg.go.dev/encore.dev/pubsub#SubscriptionConfig). After the `MaxRetries` is hit,
the event will be placed into a dead-letter queue (DLQ) for that subscriber. This will allow the subscription to continue
processing other events until such time as the bug which caused the event to fail to be processed can be fixed. At which
point, those messages can be manually released from the dead-letter queue to be processed again by the subscriber.


## Testing PubSub

In tests, Encore uses a special testing implementation of PubSub topics. When running tests topics are aware of which test
is actively running and that gives you the following guarantees:
- Your subscriptions will not be triggered by events published. This allows you to test the behaviour of publishers independently of side effects caused by subscribers.
- Message ID's generated on publish are deterministic (based on the order of publishing), thus your assertions can make use of that fact.
- Each test is isolated from other tests, meaning that events published in one test _will not_ impact another test (even if you use parallel testing).

To access the testing topic Encore provides a helper function, [`et.Topic`](https://pkg.go.dev/encore.dev/et#Topic). You
can use this object to extract the events that have been published to it during that test.

```go
package user

import (
    "testing"

    "encore.dev/et"
    "github.com/stretchr/testify/assert"
)

func Test_Register(t *testing.T) {
    t.Parallel()

    ... Call Register() and assert changes to the database ...

    // Get all published messages on the Signups topic from this test.
    msgs := et.Topic(Signups).PublishedMessages()
    assert.Len(t, msgs, 1)
}
```
