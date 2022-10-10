package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"encore.dev/appruntime/config"
	"encore.dev/pubsub/internal/types"
)

type Manager struct {
	ctx context.Context
	cfg *config.Config

	cfgOnce   sync.Once
	awsCfg    aws.Config
	snsClient *sns.Client
	sqsClient *sqs.Client
}

func NewManager(ctx context.Context, cfg *config.Config) *Manager {
	return &Manager{ctx: ctx, cfg: cfg}
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

func (mgr *Manager) NewTopic(_ *config.AWSPubsubProvider, cfg *config.PubsubTopic) types.TopicImplementation {
	snsClient := mgr.getSNSClient(mgr.ctx)
	sqsClient := mgr.getSQSClient(mgr.ctx)

	// Check we have permissions to interact with the given topic
	// otherwise the first time we will find out is when we try and publish to it
	_, err := snsClient.GetTopicAttributes(mgr.ctx, &sns.GetTopicAttributesInput{
		TopicArn: aws.String(cfg.ProviderName),
	})
	if err != nil {
		panic(fmt.Sprintf("unable to verify SNS topic attributes (may be missing IAM role allowing access): %v", err))
	}

	return &topic{mgr.ctx, snsClient, sqsClient, cfg}
}
