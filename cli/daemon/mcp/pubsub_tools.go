package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
)

func (m *Manager) registerPubSubTools() {
	m.server.AddTool(mcp.NewTool("get_pubsub",
		mcp.WithDescription("Retrieve detailed information about all PubSub topics and their subscriptions in the currently open Encore. This includes topic configurations, subscription patterns, message schemas, and the services that publish to or subscribe to each topic."),
	), m.getPubSub)

	m.server.AddTool(mcp.NewTool("wait_for_subscription_message",
		mcp.WithDescription("Block until the next message on a topic is fully processed by a subscription (handler returns or errors), then return the outcome. Use this to bridge async Pub/Sub work into a synchronous verification step instead of polling read-side endpoints in a loop."),
		mcp.WithString("topic", mcp.Description("Topic name as declared in pubsub.NewTopic. Must match the live app.")),
		mcp.WithString("subscription", mcp.Description("Subscription name to wait on. Optional — if omitted, waits for ANY subscription on the topic to process its next message.")),
		mcp.WithNumber("timeout_ms", mcp.Description("Max wait in milliseconds. Default 10000.")),
		mcp.WithString("since", mcp.Description("Optional ISO/RFC3339 timestamp. If set, return the next message processed after this time. If omitted, waits for the next message after this MCP call begins.")),
		mcp.WithObject("match", mcp.Description("Optional filter. If set, only return when a message whose JSON payload contains the given top-level key/value pairs is processed.")),
	), m.waitForSubscriptionMessage)
}

func (m *Manager) getPubSub(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Create a map to find topic and subscription definitions from trace nodes
	topicDefLocations := make(map[string]map[string]interface{})
	subscriptionDefLocations := make(map[string]map[string]map[string]interface{})

	// Scan through all packages to find trace nodes related to pubsub
	for _, pkg := range md.Pkgs {
		for _, node := range pkg.TraceNodes {
			// Check for topic definition nodes
			if node.GetPubsubTopicDef() != nil {
				topicDef := node.GetPubsubTopicDef()
				if _, exists := topicDefLocations[topicDef.TopicName]; !exists {
					topicDefLocations[topicDef.TopicName] = map[string]interface{}{
						"filepath":     node.Filepath,
						"line_start":   node.SrcLineStart,
						"line_end":     node.SrcLineEnd,
						"column_start": node.SrcColStart,
						"column_end":   node.SrcColEnd,
					}
				}
			}

			// Check for subscription definition nodes
			if node.GetPubsubSubscriber() != nil {
				subDef := node.GetPubsubSubscriber()
				if _, exists := subscriptionDefLocations[subDef.TopicName]; !exists {
					subscriptionDefLocations[subDef.TopicName] = make(map[string]map[string]interface{})
				}

				if _, exists := subscriptionDefLocations[subDef.TopicName][subDef.SubscriberName]; !exists {
					subscriptionDefLocations[subDef.TopicName][subDef.SubscriberName] = map[string]interface{}{
						"filepath":     node.Filepath,
						"line_start":   node.SrcLineStart,
						"line_end":     node.SrcLineEnd,
						"column_start": node.SrcColStart,
						"column_end":   node.SrcColEnd,
					}
				}
			}
		}
	}

	// Now build the response with locations
	topics := make([]map[string]interface{}, 0)
	for _, topic := range md.PubsubTopics {
		// Extract publishers
		publishers := make([]map[string]interface{}, 0)
		for _, publisher := range topic.Publishers {
			publishers = append(publishers, map[string]interface{}{
				"service_name": publisher.ServiceName,
			})
		}

		// Extract subscriptions
		subscriptions := make([]map[string]interface{}, 0)
		for _, subscription := range topic.Subscriptions {
			subscriptionInfo := map[string]interface{}{
				"name":         subscription.Name,
				"service_name": subscription.ServiceName,
			}

			// Add location information for subscription if available
			if subLocations, topicExists := subscriptionDefLocations[topic.Name]; topicExists {
				if subLocation, subExists := subLocations[subscription.Name]; subExists {
					subscriptionInfo["definition"] = subLocation
				}
			}

			// Add optional fields if they're set
			if subscription.AckDeadline > 0 {
				subscriptionInfo["ack_deadline"] = formatDuration(subscription.AckDeadline)
			}
			if subscription.MessageRetention > 0 {
				subscriptionInfo["message_retention"] = formatDuration(subscription.MessageRetention)
			}
			if subscription.MaxConcurrency != nil {
				subscriptionInfo["max_concurrency"] = *subscription.MaxConcurrency
			}

			// Add retry policy if available
			if subscription.RetryPolicy != nil {
				retryPolicy := map[string]interface{}{}
				if subscription.RetryPolicy.MinBackoff > 0 {
					retryPolicy["min_backoff"] = formatDuration(subscription.RetryPolicy.MinBackoff)
				}
				if subscription.RetryPolicy.MaxBackoff > 0 {
					retryPolicy["max_backoff"] = formatDuration(subscription.RetryPolicy.MaxBackoff)
				}
				if subscription.RetryPolicy.MaxRetries > 0 {
					retryPolicy["max_retries"] = subscription.RetryPolicy.MaxRetries
				}
				subscriptionInfo["retry_policy"] = retryPolicy
			}

			subscriptions = append(subscriptions, subscriptionInfo)
		}

		// Build topic info
		topicInfo := map[string]interface{}{
			"name":               topic.Name,
			"publishers":         publishers,
			"subscriptions":      subscriptions,
			"delivery_guarantee": topic.DeliveryGuarantee.String(),
		}

		// Add location information for topic if available
		if location, exists := topicDefLocations[topic.Name]; exists {
			topicInfo["definition"] = location
		}

		// Add documentation if available
		if topic.Doc != nil {
			topicInfo["doc"] = *topic.Doc
		}

		// Add ordering key if available
		if topic.OrderingKey != "" {
			topicInfo["ordering_key"] = topic.OrderingKey
		}

		// Add message type if available
		if topic.MessageType != nil {
			messageTypeData, err := protojson.Marshal(topic.MessageType)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal message type: %w", err)
			}
			var messageTypeJson interface{}
			if err := json.Unmarshal(messageTypeData, &messageTypeJson); err != nil {
				return nil, fmt.Errorf("failed to unmarshal message type JSON: %w", err)
			}
			topicInfo["message_type"] = messageTypeJson
		}

		topics = append(topics, topicInfo)
	}

	jsonData, err := json.Marshal(topics)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PubSub information: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
