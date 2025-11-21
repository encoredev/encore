---
seotitle: Using PubSub in your backend application
seodesc: Learn how you can use PubSub as an asynchronous message queue in your backend application, a great approach for decoupling services for better reliability.
title: Pub/Sub
subtitle: Decoupling services and building asynchronous systems
infobox: {
  title: "Pub/Sub Messaging",
  import: "encore.dev/pubsub",
  example_link: "/docs/ts/tutorials/uptime"
}
lang: ts
---

Publishers & Subscribers (Pub/Sub) let you build systems that communicate by broadcasting events asynchronously. This is a great way to decouple services for better reliability and responsiveness.

Encore's Backend Framework lets you use Pub/Sub in a cloud-agnostic declarative fashion. At deployment, Encore automatically [provisions the required infrastructure](/docs/platform/infrastructure/infra).

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/ts/simple-event-driven" 
    desc="Simple example app with an event-driven architecture using Pub/Sub." 
/>

## Creating a Topic

The core of Pub/Sub is the **Topic**, a named channel on which you publish events.
Topics must be declared as package level variables, and cannot be created inside functions.
Regardless of where you create a topic, it can be published to from any service, and subscribed to from any service.

When creating a topic, it must be given an event type, a unique name, and a configuration to define its behaviour.

For example, to create a topic with events about user signups:

```ts
import { Topic } from "encore.dev/pubsub"

export interface SignupEvent {
    userID: string;
}

export const signups = new Topic<SignupEvent>("signups", {
    deliveryGuarantee: "at-least-once",
});
```

## Publishing events

To publish an **Event**, call `publish` on the topic passing in the event object (which is the type specified in the `new Topic<Type>` constructor).

For example:

```ts
const messageID = await signups.publish({userID: id});

// If we get here the event has been successfully published,
// and all registered subscribers will receive the event.

// The messageID variable contains the unique id of the message,
// which is also provided to the subscribers when processing the event.
```

By defining the `signups` topic variable as an exported variable
you can also publish to the topic from other services in the same way.

## Subscribing to Events

To **Subscribe** to events, you create a Subscription as a top-level variable, by calling the
`new Subscription` constructor.

Each subscription needs:
- the topic to subscribe to
- a name which is unique for the topic
- a configuration object with at least a `handler` function to process the events
- a configuration object

For example, to create a subscription to the `signups` topic from earlier:

```ts
import { Subscription } from "encore.dev/pubsub";

const _ = new Subscription(signups, "send-welcome-email", {
    handler: async (event) => {
        // Send a welcome email using the event.
    },
});
```

Subscriptions can be defined in the same service as the topic is declared, or in any other service of your application. Each
subscription to a single topic receives the events independently of any other subscriptions to the same topic. This means
that if one subscription is running very slowly, it will grow a backlog of unprocessed events.
However, any other subscriptions will still be processing events in real-time as they are published.

### Error Handling

If a subscription function returns an error, the event being processed will be retried, based on the retry policy
configured on that subscription.

After the max number of retries is reached,the event will be placed into a dead-letter queue (DLQ) for that subscriber.
This allows the subscription to continue processing events until the bug which caused the event to fail can be fixed.
Once fixed, the messages on the dead-letter queue can be manually released to be processed again by the subscriber.

## Customizing message delivery

### At-least-once delivery

The above examples configure the topic to ensure that, for each subscription, events will be delivered _at least once_.

This means that if the topic believes the event was not processed, it will attempt to deliver the message again.
**Therefore, all subscription handlers should be [idempotent](https://en.wikipedia.org/wiki/Idempotence#Computer_science_meaning).** This helps ensure that if the handler is called two or more times, from the outside there's no difference compared to calling it once.

This can be achieved using a database to track if you have already performed the action that the event is meant to trigger,
or ensuring that the action being performed is also idempotent in nature.

### Exactly-once delivery

Topics can also be configured to deliver events _exactly once_ by setting the `deliveryGuarantee` field to
`"exactly-once"`. This enables stronger guarantees on the infrastructure level to minimize the likelihood of
message re-delivery.

However, there are still some rare circumstances when a message might be redelivered.  For example, if a networking issue
causes the acknowledgement of successful processing the message to be lost before the cloud provider receives it
(the [Two Generals' Problem](https://en.wikipedia.org/wiki/Two_Generals%27_Problem)).  As such, if correctness is critical
under all circumstances, it's still advisable to design your subscription handlers to be idempotent.

By enabling exactly-once delivery on a topic the cloud provider enforces certain throughput limitations:
- AWS: 300 messages per second for the topic (see [AWS SQS Quotas](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html)).
- GCP: At least 3,000 messages per second across all topics in the region (can be higher on the region see [GCP PubSub Quotas](https://cloud.google.com/pubsub/quotas#quotas)).

<Callout type="important">

Exactly-once delivery does not perform message deduplication on the publishing side.
If `publish` is called twice with the same message, the message will be delivered twice.

</Callout>

### Message Attributes

By default, each field in the event type is encoded as JSON and sent as part of the Pub/Sub message payload.

Pub/Sub topics also support sending data as "attributes", which are key-value
pairs that enable other behavior like subscriptions that filter messages
or ensuring message ordering.

To define that a field should be sent as an attribute, define it with the `Attribute` type.

For example, to add an attribute named `source`:

```ts
import { Topic, Attribute } from "encore.dev/pubsub";

export interface SignupEvent {
    userID: string;
    source: Attribute<string>;
}

export const signups = new Topic<SignupEvent>("signups", {
    deliveryGuarantee: "at-least-once",
});
```

### Ordered Topics

Topics are unordered by default, meaning that messages can be delivered in any order. This allows for better throughput on the topic as messages can be processed in parallel. However, in some cases, messages must be delivered in the order they were published for a given entity.

To create an ordered topic, configure the topic's `orderingAttribute` to match the name of a top-level `Attribute` field in the event type. This field ensures that messages delivered to the same subscriber are delivered in the order of publishing for that specific field value. Messages with a different value on the ordering attribute are delivered in an unspecified order.

To maintain topic order, messages with the same ordering key aren't delivered until the earliest message is processed or dead-lettered, potentially causing delays due to [head-of-line blocking](https://en.wikipedia.org/wiki/Head-of-line_blocking). Mitigate processing issues by ensuring robust logging and alerts, and appropriate subscription retry policies.

<Callout type="info">

The `orderingAttribute` currently has no effect in local environments.

</Callout>

#### Throughput limitations

Each cloud provider enforces certain throughput limitations for ordered topics:
- **AWS:** 300 messages per second for the topic (see [AWS SQS Quotas](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html))
- **GCP:** 1 MBps for each ordering key (See [GCP Pub/Sub Resource Limits](https://cloud.google.com/pubsub/quotas#resource_limits))

#### Ordered topic example

```ts
import { Topic, Attribute } from "encore.dev/pubsub";

export interface CartEvent {
	shoppingCartID: Attribute<number>;
	event: string;
}

export const cartEvents = new Topic<CartEvent>("cart-events", {
	deliveryGuarantee: "at-least-once",
	orderingAttribute: "shoppingCartID",
})

async function example() {
	// These are delivered in order as they all have the same shopping cart ID
	await cartEvents.publish({shoppingCartID: 1, event: "item_added"});
	await cartEvents.publish({shoppingCartID: 1, event: "checkout_started"});
	await cartEvents.publish({shoppingCartID: 1, event: "checkout_completed"});

	// This may be delivered at any point as it has a different shopping cart ID.
	await cartEvents.publish({shoppingCartID: 2, event: "item_added"});
}
```

## Topic references

Encore uses static analysis to determine which services are accessing each Pub/Sub topic,
and what operations each service is performing.

That information is used for features such as rendering architecture diagrams, and is used by Encore Cloud to provision infrastructure correctly and configure IAM permissions.

This means `Topic` objects can't be passed around however you like,
as it makes static analysis impossible in many cases. To simplify your workflow, given these restrictions,
Encore supports defining a "reference" to a topic that can be passed around any way you want.

### Using topic references

Define a topic reference by calling `topic.ref<DesiredPermissions>()` from within a service, where `DesiredPermissions` is one of the pre-defined permission types defined in the `encore.dev/pubsub` module. 

This means you're effectively pre-declaring the permissions you need, and only the methods that
are allowed by those permissions are available on the returned reference object.

For example, to get a reference to a topic that can publish messages:

```typescript
import { Publisher } from "encore.dev/pubsub";
const ref = cartEvents.ref<Publisher>();

// You can now freely pass around `ref`, and you can use
// `ref.publish()` just like you would `cartEvents.publish()`.
```

To ensure Encore still is aware of which permissions each service needs, the call to `topic.ref`
must be made from within a service, so that Encore knows which service to associate the permissions with.

Currently, the only permission type is `Publisher`, which allows publishing events to the topic.
We plan to add more permission types in the future.
