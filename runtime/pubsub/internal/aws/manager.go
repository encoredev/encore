package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/rs/xid"

	"encore.dev/appruntime/exported/config"
	"encore.dev/pubsub/internal/types"
)

type Manager struct {
	ctx context.Context

	// publisherID is a unique ID for this Encore app instance, used as the Message Group ID
	// for topics which don't specify a grouping field. This is based on [AWS's recommendation]
	// that each producer should have a unique message group ID to send all it's messages.
	//
	// We use an XID here as we've already got the library included within the runtime and it has an excellent
	// way of generating unique IDs in a distributed system without needing to talk to a central service.
	//
	// [AWS's recommendation]: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/FIFO-queues-understanding-logic.html
	publisherID xid.ID

	cfgOnce   sync.Once
	awsCfg    aws.Config
	snsClient *sns.Client
	sqsClient *sqs.Client
}

func NewManager(ctx context.Context) *Manager {
	return &Manager{ctx: ctx, publisherID: xid.New()}
}

func (mgr *Manager) ProviderName() string { return "aws" }

func (mgr *Manager) Matches(cfg *config.PubsubProvider) bool {
	return cfg.AWS != nil
}

// getConfig loads the required AWS config to connect to AWS
func (mgr *Manager) getConfig(ctx context.Context) aws.Config {
	mgr.cfgOnce.Do(func() {
		cfg, err := awsConfig.LoadDefaultConfig(ctx)
		if err != nil {
			panic(fmt.Sprintf("unable to load AWS config: %v", err))
		}
		mgr.awsCfg = cfg

	})

	return mgr.awsCfg
}

func (mgr *Manager) getSNSClient(ctx context.Context) *sns.Client {
	if mgr.snsClient == nil {
		mgr.snsClient = sns.NewFromConfig(mgr.getConfig(ctx))
	}
	return mgr.snsClient
}

func (mgr *Manager) getSQSClient(ctx context.Context) *sqs.Client {
	if mgr.sqsClient == nil {
		mgr.sqsClient = sqs.NewFromConfig(mgr.getConfig(ctx))
	}
	return mgr.sqsClient
}

func (mgr *Manager) NewTopic(_ *config.PubsubProvider, staticCfg types.TopicConfig, runtimeCfg *config.PubsubTopic) types.TopicImplementation {
	snsClient := mgr.getSNSClient(mgr.ctx)
	sqsClient := mgr.getSQSClient(mgr.ctx)

	// Check we have permissions to interact with the given topic
	// otherwise the first time we will find out is when we try and publish to it
	_, err := snsClient.GetTopicAttributes(mgr.ctx, &sns.GetTopicAttributesInput{
		TopicArn: aws.String(runtimeCfg.ProviderName),
	})
	if err != nil {
		panic(fmt.Sprintf("unable to verify SNS topic attributes (may be missing IAM role allowing access): %v", err))
	}

	return &topic{mgr.ctx, mgr.publisherID, snsClient, sqsClient, staticCfg, runtimeCfg}
}
