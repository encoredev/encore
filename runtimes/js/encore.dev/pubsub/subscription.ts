import { setCurrentRequest } from "../internal/reqtrack/mod";
import { DurationString } from "../internal/types/mod";
import { Topic } from "./topic";
import * as runtime from "../internal/runtime/mod";

export class Subscription<Msg extends object> {
  private readonly topic: Topic<Msg>;
  private readonly name: string;
  private readonly impl: runtime.PubSubSubscription;

  constructor(topic: Topic<Msg>, name: string, cfg: SubscriptionConfig<Msg>) {
    this.topic = topic;
    this.name = name;

    const handler = (msg: runtime.Request) => {
      setCurrentRequest(msg);
      return cfg.handler(msg.payload() as Msg);
    };

    this.impl = runtime.RT.pubsubSubscription({
      topicName: topic.name,
      subscriptionName: name,
      handler,
    });

    this.startSubscribing();
  }

  private startSubscribing() {
    const that = this;
    this.impl.subscribe().finally(() => {
      setTimeout(() => that.startSubscribing(), 1000);
    });
  }
}

/**
 * SubscriptionConfig is used when creating a subscription
 *
 * The values given here may be clamped to the supported values by
 * the target cloud. (i.e. ack deadline may be brought within the supported range
 * by the target cloud pubsub implementation).
 */
export interface SubscriptionConfig<Msg> {
  /**
   * Handler is the function which will be called to process a message
   * sent on the topic.
   *
   * When this function returns an error the message will be
   * negatively acknowledged (nacked), which will cause a redelivery
   * attempt to be made (unless the retry policy's MaxRetries has been reached).
   */
  handler: (msg: Msg) => Promise<unknown>;

  /**
   * MaxConcurrency is the maximum number of messages which will be processed
   * simultaneously per instance of the service for this subscription.
   *
   * Note that this is per instance of the service, so if your service has
   * scaled to 10 instances and this is set to 10, then 100 messages could be
   * processed simultaneously.
   *
   * If the value is negative, then there will be no limit on the number
   * of messages processed simultaneously.
   *
   * Note: This is not supported by all cloud providers; specifically on GCP
   * when using Cloud Run instances on an unordered topic the subscription will
   * be configured as a Push Subscription and will have an adaptive concurrency
   * See [GCP Push Delivery Rate](https://cloud.google.com/pubsub/docs/push#push_delivery_rate).
   *
   * This setting also has no effect on Encore Cloud environments.
   * If not set, it uses a reasonable default based on the cloud provider.
   */
  maxConcurrency?: number;

  /**
   * AckDeadline is the time a consumer has to process a message
   * before it's returned to the subscription
   *
   * Default is 30 seconds, however the ack deadline must be at least
   * 1 second.
   */
  ackDeadline?: DurationString;

  /**
   * MessageRetention is how long an undelivered message is kept
   * on the topic before it's purged.
   *
   * Default is 7 days.
   */
  messageRetention?: DurationString;

  /**
   * RetryPolicy defines how a message should be retried when
   * the subscriber returns an error
   */
  retryPolicy?: RetryPolicy;
}

/**
 * RetryPolicy defines how a subscription should handle retries
 * after errors either delivering the message or processing the message.
 *
 * The values given to this structure are parsed at compile time, such that
 * the correct Cloud resources can be provisioned to support the queue.
 *
 * As such the values given here may be clamped to the supported values by
 * the target cloud. (i.e. min/max values brought within the supported range
 * by the target cloud).
 */
export interface RetryPolicy {
  /**
   * The minimum time to wait between retries. Defaults to 10 seconds.
   */
  minBackoff?: DurationString;

  /**
   * The maximum time to wait between retries. Defaults to 10 minutes.
   */
  maxBackoff?: DurationString;

  /**
   * MaxRetries is used to control deadletter queuing logic, when:
   *   n == 0: A default value of 100 retries will be used
   *   n > 0:  Encore will forward a message to a dead letter queue after n retries
   *   n == pubsub.InfiniteRetries: Messages will not be forwarded to the dead letter queue by the Encore framework
   */
  maxRetries?: number;
}
