import { getCurrentRequest } from "../internal/reqtrack/mod";
import type { AttributesOf } from "./mod";
import * as runtime from "../internal/runtime/mod";

/**
 * A topic is a resource to which you can publish messages
 * to be delivered to subscribers of that topic.
 */
export class Topic<Msg extends object> {
  public readonly name: string;
  public readonly cfg: TopicConfig<Msg>;
  private impl: runtime.PubSubTopic;

  constructor(name: string, cfg: TopicConfig<Msg>) {
    this.name = name;
    this.cfg = cfg;
    this.impl = runtime.RT.pubsubTopic(name);
  }

  public async publish(msg: Msg): Promise<string> {
    const source = getCurrentRequest();
    return this.impl.publish(msg, source);
  }
}

/**
 * DeliveryGuarantee is used to configure the delivery contract for a topic.
 */
export type DeliveryGuarantee = "at-least-once" | "exactly-once";

/**
 * At Least Once delivery guarantees that a message for a subscription is delivered to
 * a consumer at least once.
 *
 * On AWS and GCP there is no limit to the throughput for a topic.
 */
export const atLeastOnce: DeliveryGuarantee = "at-least-once";

/**
 * ExactlyOnce guarantees that a message for a subscription is delivered to
 * a consumer exactly once, to the best of the system's ability.
 *
 * However, there are edge cases when a message might be redelivered.
 * For example, if a networking issue causes the acknowledgement of success
 * processing the message to be lost before the cloud provider receives it.
 *
 * It is also important to note that the ExactlyOnce delivery guarantee only
 * applies to the delivery of the message to the consumer, and not to the
 * original publishing of the message, such that if a message is published twice,
 * such as due to an retry within the application logic, it will be delivered twice.
 * (i.e. ExactlyOnce delivery does not imply message deduplication on publish)
 *
 * As such it's recommended that the subscription handler function is idempotent
 * and is able to handle duplicate messages.
 *
 * Subscriptions attached to ExactlyOnce topics have higher message delivery latency compared to AtLeastOnce.
 *
 * By using ExactlyOnce semantics on a topic, the throughput will be limited depending on the cloud provider:
 * - AWS: 300 messages per second for the topic (see [AWS SQS Quotas]).
 * - GCP: At least 3,000 messages per second across all topics in the region
 *      (can be higher on the region see [GCP PubSub Quotas]).
 *
 * [AWS SQS Quotas]: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html
 * [GCP PubSub Quotas]: https://cloud.google.com/pubsub/quotas#quotas
 */
export const exactlyOnce: DeliveryGuarantee = "exactly-once";

/**
 * TopicConfig is used when creating a Topic
 */
export interface TopicConfig<Msg extends object> {
  /**
   * DeliveryGuarantee is used to configure the delivery guarantee of a Topic
   */
  deliveryGuarantee: DeliveryGuarantee;

  /**
   * OrderingAttribute is the message attribute to use as a ordering key for
   * messages and delivery will ensure that messages with the same value will
   * be delivered in the order they where published.
   *
   * If OrderingAttribute is not set, messages can be delivered in any order.
   *
   * It is important to note, that in the case of an error being returned by a
   * subscription handler, the message will be retried before any subsequent
   * messages for that ordering key are delivered. This means depending on the
   * retry configuration, a large backlog of messages for a given ordering key
   * may build up. When using OrderingAttribute, it is recommended to use reason
   * about your failure modes and set the retry configuration appropriately.
   *
   * Once the maximum number of retries has been reached, the message will be
   * forwarded to the dead letter queue, and the next message for that ordering
   * key will be delivered.
   *
   * To create attributes on a message, use the `Attribute` type:
   *
   *  type UserEvent = {
   *    user_id: Attribute<string>;
   *    action:  string;
   *  }
   *
   *  const topic = new Topic<UserEvent>("user-events", {
   *    deliveryGuarantee: DeliveryGuarantee.AtLeastOnce,
   *    orderingAttribute: "user_id", // Messages with the same user-id will be delivered in the order they where
   * published
   *  })
   *
   *  topic.publish(ctx, {user_id: "1", action: "login"})  // This message will be delivered before the logout
   *  topic.publish(ctx, {user_id: "2", action: "login"})  // This could be delivered at any time because it has a different user id
   *  topic.publish(ctx, {user_id: "1", action: "logout"}) // This message will be delivered after the first message
   *
   * By using OrderingAttribute, the throughput will be limited depending on the cloud provider:
   *
   * - AWS: 300 messages per second for the topic (see [AWS SQS Quotas]).
   * - GCP: 1MB/s for each ordering key (see [GCP PubSub Quotas]).
   *
   * Note: OrderingAttribute currently has no effect during local development.
   *
   * [AWS SQS Quotas]: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html
   * [GCP PubSub Quotas]: https://cloud.google.com/pubsub/quotas#resource_limits
   */
  orderingAttribute?: AttributesOf<Msg>;
}
