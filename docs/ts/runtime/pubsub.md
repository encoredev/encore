---
title: encore.dev/pubsub
lang: ts
toc: true
---

# encore.dev/pubsub

## Classes

<!-- symbol-start: Subscription -->
### Subscription

<!-- source: pubsub/subscription.ts:6 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L6)

#### Type Parameters

##### Msg

`Msg` *extends* `object`

#### Constructors

##### Constructor

```ts
new Subscription<Msg>(
   topic, 
   name, 
cfg): Subscription<Msg>;
```

<!-- source: pubsub/subscription.ts:11 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L11)

###### Parameters

###### topic

[`Topic`](#topic)\<`Msg`\>

###### name

`string`

###### cfg

[`SubscriptionConfig`](#subscriptionconfig)\<`Msg`\>

###### Returns

[`Subscription`](#subscription)\<`Msg`\>

***

<!-- symbol-end -->

<!-- symbol-start: Topic -->
### Topic

<!-- source: pubsub/topic.ts:10 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L10)

A topic is a resource to which you can publish messages
to be delivered to subscribers of that topic.

#### Extends

- [`TopicPerms`](#topicperms)

#### Type Parameters

##### Msg

`Msg` *extends* `object`

#### Implements

- [`Publisher`](#publisher)\<`Msg`\>

#### Constructors

##### Constructor

`new Topic<Msg>(name, cfg): Topic<Msg>`

<!-- source: pubsub/topic.ts:18 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L18)

###### Parameters

###### name

`string`

###### cfg

[`TopicConfig`](#topicconfig)\<`Msg`\>

###### Returns

[`Topic`](#topic)\<`Msg`\>

###### Overrides

`TopicPerms.constructor`

#### Properties

##### cfg

`readonly cfg: TopicConfig<Msg>`

<!-- source: pubsub/topic.ts:15 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L15)

##### name

`readonly name: string`

<!-- source: pubsub/topic.ts:14 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L14)

#### Methods

##### publish()

`publish(msg): Promise<string>`

<!-- source: pubsub/topic.ts:25 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L25)

###### Parameters

###### msg

`Msg`

###### Returns

`Promise`\<`string`\>

###### Implementation of

[`Publisher`](#publisher).[`publish`](#publish-1)

##### ref()

`ref<P>(): P`

<!-- source: pubsub/topic.ts:30 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L30)

###### Type Parameters

###### P

`P` *extends* [`TopicPerms`](#topicperms)

###### Returns

`P`

<!-- symbol-end -->

## Interfaces

<!-- symbol-start: Publisher -->
### Publisher

<!-- source: pubsub/refs.ts:5 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/refs.ts#L5)

#### Extends

- [`TopicPerms`](#topicperms)

#### Type Parameters

##### Msg

`Msg` *extends* `object`

#### Methods

##### publish()

`abstract publish(msg): Promise<string>`

<!-- source: pubsub/refs.ts:6 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/refs.ts#L6)

###### Parameters

###### msg

`Msg`

###### Returns

`Promise`\<`string`\>

***

<!-- symbol-end -->

<!-- symbol-start: RetryPolicy -->
### RetryPolicy

<!-- source: pubsub/subscription.ts:111 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L111)

RetryPolicy defines how a subscription should handle retries
after errors either delivering the message or processing the message.

The values given to this structure are parsed at compile time, such that
the correct Cloud resources can be provisioned to support the queue.

As such the values given here may be clamped to the supported values by
the target cloud. (i.e. min/max values brought within the supported range
by the target cloud).

#### Properties

##### maxBackoff?

`optional maxBackoff?: DurationString`

<!-- source: pubsub/subscription.ts:120 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L120)

The maximum time to wait between retries. Defaults to 10 minutes.

##### maxRetries?

`optional maxRetries?: number`

<!-- source: pubsub/subscription.ts:128 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L128)

MaxRetries is used to control deadletter queuing logic, when:
  n == 0: A default value of 100 retries will be used
  n > 0:  Encore will forward a message to a dead letter queue after n retries
  n == pubsub.InfiniteRetries: Messages will not be forwarded to the dead letter queue by the Encore framework

##### minBackoff?

`optional minBackoff?: DurationString`

<!-- source: pubsub/subscription.ts:115 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L115)

The minimum time to wait between retries. Defaults to 10 seconds.

***

<!-- symbol-end -->

<!-- symbol-start: SubscriptionConfig -->
### SubscriptionConfig

<!-- source: pubsub/subscription.ts:44 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L44)

SubscriptionConfig is used when creating a subscription

The values given here may be clamped to the supported values by
the target cloud. (i.e. ack deadline may be brought within the supported range
by the target cloud pubsub implementation).

#### Type Parameters

##### Msg

`Msg`

#### Properties

##### ackDeadline?

`optional ackDeadline?: DurationString`

<!-- source: pubsub/subscription.ts:83 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L83)

AckDeadline is the time a consumer has to process a message
before it's returned to the subscription

Default is 30 seconds, however the ack deadline must be at least
1 second.

##### handler

`handler: (msg) => Promise<unknown>`

<!-- source: pubsub/subscription.ts:53 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L53)

Handler is the function which will be called to process a message
sent on the topic.

When this function returns an error the message will be
negatively acknowledged (nacked), which will cause a redelivery
attempt to be made (unless the retry policy's MaxRetries has been reached).

###### Parameters

###### msg

`Msg`

###### Returns

`Promise`\<`unknown`\>

##### maxConcurrency?

`optional maxConcurrency?: number`

<!-- source: pubsub/subscription.ts:74 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L74)

MaxConcurrency is the maximum number of messages which will be processed
simultaneously per instance of the service for this subscription.

Note that this is per instance of the service, so if your service has
scaled to 10 instances and this is set to 10, then 100 messages could be
processed simultaneously.

If the value is negative, then there will be no limit on the number
of messages processed simultaneously.

Note: This is not supported by all cloud providers; specifically on GCP
when using Cloud Run instances on an unordered topic the subscription will
be configured as a Push Subscription and will have an adaptive concurrency
See [GCP Push Delivery Rate](https://cloud.google.com/pubsub/docs/push#push_delivery_rate).

This setting also has no effect on Encore Cloud environments.
If not set, it uses a reasonable default based on the cloud provider.

##### messageRetention?

`optional messageRetention?: DurationString`

<!-- source: pubsub/subscription.ts:91 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L91)

MessageRetention is how long an undelivered message is kept
on the topic before it's purged.

Default is 7 days.

##### retryPolicy?

`optional retryPolicy?: RetryPolicy`

<!-- source: pubsub/subscription.ts:97 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/subscription.ts#L97)

RetryPolicy defines how a message should be retried when
the subscriber returns an error

***

<!-- symbol-end -->

<!-- symbol-start: TopicConfig -->
### TopicConfig

<!-- source: pubsub/topic.ts:80 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L80)

TopicConfig is used when creating a Topic

#### Type Parameters

##### Msg

`Msg` *extends* `object`

#### Properties

##### deliveryGuarantee

`deliveryGuarantee: DeliveryGuarantee`

<!-- source: pubsub/topic.ts:84 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L84)

DeliveryGuarantee is used to configure the delivery guarantee of a Topic

##### orderingAttribute?

`optional orderingAttribute?: keyof { [Key in string | number | symbol as Extract<Msg[Key], brandedAttribute<string> | brandedAttribute<number> | brandedAttribute<false> | brandedAttribute<true>> extends never ? never : Key]: never }`

<!-- source: pubsub/topic.ts:131 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L131)

OrderingAttribute is the message attribute to use as a ordering key for
messages and delivery will ensure that messages with the same value will
be delivered in the order they where published.

If OrderingAttribute is not set, messages can be delivered in any order.

It is important to note, that in the case of an error being returned by a
subscription handler, the message will be retried before any subsequent
messages for that ordering key are delivered. This means depending on the
retry configuration, a large backlog of messages for a given ordering key
may build up. When using OrderingAttribute, it is recommended to use reason
about your failure modes and set the retry configuration appropriately.

Once the maximum number of retries has been reached, the message will be
forwarded to the dead letter queue, and the next message for that ordering
key will be delivered.

To create attributes on a message, use the `Attribute` type:

 type UserEvent = {
   user_id: Attribute<string>;
   action:  string;
 }

 const topic = new Topic<UserEvent>("user-events", {
   deliveryGuarantee: DeliveryGuarantee.AtLeastOnce,
   orderingAttribute: "user_id", // Messages with the same user-id will be delivered in the order they where
published
 })

 topic.publish(ctx, {user_id: "1", action: "login"})  // This message will be delivered before the logout
 topic.publish(ctx, {user_id: "2", action: "login"})  // This could be delivered at any time because it has a different user id
 topic.publish(ctx, {user_id: "1", action: "logout"}) // This message will be delivered after the first message

By using OrderingAttribute, the throughput will be limited depending on the cloud provider:

- AWS: 300 messages per second for the topic (see [AWS SQS Quotas]).
- GCP: 1MB/s for each ordering key (see [GCP PubSub Quotas]).

Note: OrderingAttribute currently has no effect during local development.

[AWS SQS Quotas]: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html
[GCP PubSub Quotas]: https://cloud.google.com/pubsub/quotas#resource_limits

***

<!-- symbol-end -->

<!-- symbol-start: TopicPerms -->
### TopicPerms

<!-- source: pubsub/refs.ts:1 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/refs.ts#L1)

#### Extended by

- [`Topic`](#topic)
- [`Publisher`](#publisher)

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: Attribute -->
### Attribute

`type Attribute<T> = T | brandedAttribute<T>`

<!-- source: pubsub/mod.ts:35 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/mod.ts#L35)

Attribute represents a field on a message that should be sent
as an attribute in a PubSub message, rather than in the message
body.

This is useful for ordering messages, or for filtering messages
on a subscription - otherwise you should not use this.

To create attributes on a message, use the `Attribute` type:
  type Message = {
    user_id: Attribute<number>;
    name: string;
  };

  const msg: Message = {
    user_id: 123,
    name:    "John Doe",
  };

The union of brandedAttribute is simply used to help the TypeScript compiler
understand that the type is an attribute and allow the AttributesOf type
to extract the keys of said type.

#### Type Parameters

##### T

`T` *extends* `string` \| `number` \| `boolean`

***

<!-- symbol-end -->

<!-- symbol-start: AttributesOf -->
### AttributesOf

`type AttributesOf<T> = keyof { [Key in keyof T as Extract<T[Key], allBrandedTypes> extends never ? never : Key]: never }`

<!-- source: pubsub/mod.ts:52 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/mod.ts#L52)

AttributesOf is a helper type to extract all keys from an object
who's type is an Attribute type.

For example:
   type Message = {
       user_id: Attribute<number>;
       name: string;
       age: Attribute<number>;
   };

   type MessageAttributes = AttributesOf<Message>; // "user_id" | "age"

#### Type Parameters

##### T

`T` *extends* `object`

***

<!-- symbol-end -->

<!-- symbol-start: DeliveryGuarantee -->
### DeliveryGuarantee

`type DeliveryGuarantee = "at-least-once" | "exactly-once"`

<!-- source: pubsub/topic.ts:38 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/pubsub/topic.ts#L38)

DeliveryGuarantee is used to configure the delivery contract for a topic.


<!-- symbol-end -->