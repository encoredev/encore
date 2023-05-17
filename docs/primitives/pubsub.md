---
seotitle: Using PubSub in your backend application
seodesc: Learn how you can use PubSub as an asynchronous message queue in your backend application, a great approach for decoupling services for better reliability.
title: Pub/Sub
subtitle: Decoupling services and building asynchronous systems
infobox: {
  title: "Pub/Sub Messaging",
  import: "encore.dev/pubsub",
  example_link: "/docs/tutorials/uptime"
}
---

Publishers & Subscribers (Pub/Sub) let you build systems that communicate by broadcasting events asynchronously. This is a great way to decouple services for better reliability and responsiveness.

Encore's Infrastructure SDK lets you use Pub/Sub in a cloud-agnostic declarative fashion. At deployment, Encore automatically [provisions the required infrastructure](/docs/deploy/infra).

## Creating a Topic

The core of Pub/Sub is the **Topic**, a named channel on which you publish events.
Topics must be declared as package level variables, and cannot be created inside functions.
Regardless of where you create a topic, it can be published to from any service, and subscribed to from any service.

When creating a topic, it must be given an event type, a unique name, and a configuration to define its behaviour. See the complete specification in the [package documentation](https://pkg.go.dev/encore.dev/pubsub#NewTopic).

For example, to create a topic with events about user signups:

```go
package user

import "encore.dev/pubsub"

type SignupEvent struct{ UserID int }

var Signups = pubsub.NewTopic[*SignupEvent]("signups", pubsub.TopicConfig{
    DeliveryGuarantee: pubsub.AtLeastOnce,
})
```

### At-least-once delivery

The above example configures the topic to ensure that, for each subscription, events will be delivered _at least once_.

This means that if the topic believes the event was not processed, it will attempt to deliver the message again.
**Therefore, all subscription handlers should be [idempotent](https://en.wikipedia.org/wiki/Idempotence#Computer_science_meaning).** This helps ensure that if the handler is called two or more times, from the outside there's no difference compared to calling it once.

This can be achieved using a database to track if you have already performed the action that the event is meant to trigger,
or ensuring that the action being performed is also idempotent in nature.

### Exactly-once delivery

Topics can also be configured to deliver events _exactly once_ by setting the `DeliveryGuarantee` field to
`pubsub.ExactlyOnce`. This enables stronger guarantees on the infrastructure level to minimize the likelihood of
message re-delivery.


However, there are still some rare circumstances when a message might be redelivered.  For example, if a networking issue
causes the acknowledgement of successful processing the message to be lost before the cloud provider receives it
(the [Two Generals' Problem](https://en.wikipedia.org/wiki/Two_Generals%27_Problem)).  As such, if correctness is critical
under all circumstances, it's still advisable to design your subscription handlers to be idempotent.

By enabling exactly-once delivery on a topic the cloud provider enforces certain throughput limitations:
- AWS: 300 messages per second for the topic (see [AWS SQS Quotas](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html)).
- GCP: At least 3,000 messages per second across all topics in the region (can be higher on the region see [GCP PubSub Quotas](https://cloud.google.com/pubsub/quotas#quotas)).

<Callout type="important">

Exactly-once delivery does not perform message deduplication on the publishing side. If  `Publish` is called twice with
the same message, the message will be delivered twice.

</Callout>

### Ordered Topics

Topics are unordered by default, meaning that messages can be delivered in any order. This allows for better throughput on the topic as messages can be processed in parallel. However, in some cases, messages must be delivered in the order they were published for a given entity.

To create an ordered topic, configure the topic's `OrderingAttribute` to match the `pubsub-attr` tag on one of the top-level fields of the event type. This field ensures that messages delivered to the same subscriber are delivered in the order of publishing for that specific field value. Messages with a different value on the ordering attribute are delivered in an unspecified order.

To maintain topic order, messages with the same ordering key aren't delivered until the earliest message is processed or dead-lettered, potentially causing delays due to [head-of-line blocking](https://en.wikipedia.org/wiki/Head-of-line_blocking). Mitigate processing issues by ensuring robust logging and alerts, and appropriate subscription retry policies.

#### Throughput limitations

Each cloud provider enforces certain throughput limitations for ordered topics:
- **AWS:** 300 messages per second for the topic (see [AWS SQS Quotas](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html)).
- **GCP:** 1 MBps for each ordering key (See [GCP PubSub Resource Limits](https://cloud.google.com/pubsub/quotas#resource_limits))

<Callout type="info">

The OrderingAttribute currently has no effect in local environments.

</Callout>

<Toggle label="Example Ordered Topic">

```go
package example

import (
	"context"
	"encore.dev/pubsub"
)

type CartEvent struct {
	ShoppingCartID int `pubsub-attr:"cart_id"`
	Event          string
}

var CartEvents = pubsub.NewTopic[*CartEvent]("cart-events", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
	OrderingAttribute: "cart_id",
})

func Example(ctx context.Context) error {
	// The following events will be delivered in order as they all have the same
	// shopping cart ID
	CartEvents.Publish(ctx, &CartEvent{ShoppingCartID: 1, Event: "item_added"})
	CartEvents.Publish(ctx, &CartEvent{ShoppingCartID: 1, Event: "checkout_started"})
	CartEvents.Publish(ctx, &CartEvent{ShoppingCartID: 1, Event: "checkout_completed"})

	// However this event could be delievered at any point as it has a different shopping
	// cart ID
	CartEvents.Publish(ctx, &CartEvent{ShoppingCartID: 2, Event: "item_added"})
}
```

</Toggle>




## Publishing events

To publish an **Event**, call `Publish` on the topic passing in the event object (which is the type specified in the `pubsub.NewTopic[Type]` constructor).

For example:

```go
messageID, err := Signups.Publish(ctx, &SignupEvent{UserID: id})
if err != nil {
    return err
}

// If we get here the event has been successfully published,
// and all registered subscribers will receive the event.

// The messageID variable contains the unique id of the message,
// which is also provided to the subscribers when processing the event.
```

By defining the `Signups` topic variable as an exported variable
you can also publish to the topic from other services in the same way.

### Using topic references

Encore uses static analysis to determine which services are publishing messages
to what topics. That information is used to provision infrastructure correctly,
render architecture diagrams, and configure IAM permissions.

This means that `*pubsub.Topic` variables can't be passed around however you'd like,
as it makes static analysis impossible in many cases. To work around these restrictions
Encore allows you to get a reference to a topic that can be passed around any way you want.

It looks like this (using the `Signups` topic above):

```go
signupRef := pubsub.TopicRef[pubsub.Publisher[*SignupEvent]](Signups)

// signupRef is of type pubsub.Publisher[*SignupEvent], which allows publishing.
```

The difference between a **TopicRef** and a **Topic** is that topic references need to pre-declare
what permissions are needed. Encore then assumes that all the permissions you declare are used.

For example, if you declare a **TopicRef** with the `pubsub.Publisher` permission (as seen above)
Encore assumes that the service will publish messages to the topic and provisions the infrastructure
to support that.

Note that a **TopicRef** must be declared _within a service_, but the reference itself
can be freely passed around to library code, be dependency injected into [service structs](/docs/how-to/dependency-injection),
and so on.

## Subscribing to Events

To **Subscribe** to events, you create a Subscription as a package level variable by calling the
[`pubsub.NewSubscription`](https://pkg.go.dev/encore.dev/pubsub#NewSubscription) function.

Each subscription needs:
- the topic to subscribe to
- a name which is unique for the topic
- a configuration object with at least a `Handler` function to process the events
- a configuration object

Here's an example of how you create a subscription to a topic:

```go
package email

import (
    "encore.dev/pubsub"
    "user"
)

var _ = pubsub.NewSubscription(
    user.Signups, "send-welcome-email",
    pubsub.SubscriptionConfig[*SignupEvent]{
        Handler: SendWelcomeEmail,
    },
)
func SendWelcomeEmail(ctx context.Context, event *SignupEvent) error {
    // send email...
    return nil
}
```

Subscriptions can be in the same service as the topic is declared, or in any other service of your application. Each
subscription to a single topic receives the events independently of any other subscriptions to the same topic. This means
that if one subscription is running very slowly, it will grow a backlog of unprocessed events. However, any other subscriptions will still be processing events in real-time as they are published.

### Subscription configuration

When creating a subscription you can configure behavior such as message retention and retry policy, using the `SubscriptionConfig` type. See the [package documentation](https://pkg.go.dev/encore.dev/pubsub#SubscriptionConfig) for the complete configuration options.

<Callout type="info">

The `SubscriptionConfig` struct fields must be defined as compile-time constants, and cannot be defined in
terms of function calls. This is necessary for Encore to understand the exact requirements of the subscription, in order to provision the correct infrastructure upon deployment.

</Callout>

### Error Handling

If a subscription function returns an error, the event being processed will be retried, based on the retry policy
[configured on that subscription](https://pkg.go.dev/encore.dev/pubsub#SubscriptionConfig). After the `MaxRetries` is hit,
the event will be placed into a dead-letter queue (DLQ) for that subscriber. This allows the subscription to continue
processing events until the bug which caused the event to fail can be fixed. Once fixed, the messages on the dead-letter queue can be manually released to be processed again by the subscriber.

## Testing Pub/Sub

Encore uses a special testing implementation of Pub/Sub topics. When running tests, topics are aware of which test
is running. This gives you the following guarantees:
- Your subscriptions will not be triggered by events published. This allows you to test the behaviour of publishers independently of side effects caused by subscribers.
- Message ID's generated on publish are deterministic (based on the order of publishing), thus your assertions can make use of that fact.
- Each test is isolated from other tests, meaning that events published in one test will not impact other tests (even if you use parallel testing).

Encore provides a helper function, [`et.Topic`](https://pkg.go.dev/encore.dev/et#Topic), to access the testing topic. You
can use this object to extract the events that have been published to it during a test.

Here's an example implementation:

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

## The benefits of Pub/Sub

Pub/Sub is a powerful building block in a backend application. It can be used to improve app reliability by reducing the blast radius of faulty components and bottlenecks. It can also be used to increase the speed of response to the user, and even helps reduce cognitive overhead for developers by inverting the dependencies between services.

For those not familiar with Pub/Sub, lets take a look at an example API in a user registration service.
The behavior we want to implement is that upon registration, we send a welcome email to the user and create a record of the signup in our analytics system. Now let's see how we could implement this only using APIs, compared to how a Pub/Sub implementation might look.

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

### A Pub/Sub approach

A more ideal solution would be if we could decouple the behaviour of emailing the user and recording our analytics, such that
the user service only has to record the user in its own database and let the user know they are registered - without worrying
about the downstream impacts. Thankfully, this is exactly what [Pub/Sub topics](https://pkg.go.dev/encore.dev/pubsub#Topic) allow us to do.

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

Notice how in this version, the processing time of the two other services did not impact the end user and in fact the `user`
service is not even aware of the `email` and `analytics` services. This means that new systems which need to know about
new users signing up can be added to the application, without the need to change the `user` service or impacting its
performance.
