package infra

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// TestAzureBlob_Validate tests validation of AzureBlob configurations.
func TestAzureBlob_Validate(t *testing.T) {
	tests := []struct {
		name      string
		azureBlob *AzureBlob
		wantErr   bool
		errField  string
	}{
		{
			name: "valid config",
			azureBlob: &AzureBlob{
				StorageAccount: "mystorageaccount",
				Buckets:        map[string]*Bucket{"bucket1": {Name: "container1"}},
			},
			wantErr: false,
		},
		{
			name: "empty storage account",
			azureBlob: &AzureBlob{
				StorageAccount: "",
				Buckets:        map[string]*Bucket{"bucket1": {Name: "container1"}},
			},
			wantErr:  true,
			errField: "storage_account",
		},
		{
			name: "no buckets is valid",
			azureBlob: &AzureBlob{
				StorageAccount: "mystorageaccount",
				Buckets:        map[string]*Bucket{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			_, errs := Validate(tt.azureBlob)

			if tt.wantErr {
				c.Assert(len(errs) > 0, qt.IsTrue, qt.Commentf("expected validation errors"))
				if tt.errField != "" {
					found := false
					for path := range errs {
						if path.String() == "."+tt.errField {
							found = true
							break
						}
					}
					c.Assert(found, qt.IsTrue, qt.Commentf("expected error for field %q", tt.errField))
				}
			} else {
				c.Assert(len(errs), qt.Equals, 0, qt.Commentf("unexpected errors: %v", errs))
			}
		})
	}
}

// TestAzureServiceBusPubsub_Validate tests validation of Azure Service Bus configurations.
func TestAzureServiceBusPubsub_Validate(t *testing.T) {
	tests := []struct {
		name      string
		pubsub    *AzureServiceBusPubsub
		wantErr   bool
		errField  string
	}{
		{
			name: "valid config",
			pubsub: &AzureServiceBusPubsub{
				Namespace: "my-namespace",
				Topics: map[string]*AzureTopic{
					"topic1": {Name: "azure-topic-1"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty namespace",
			pubsub: &AzureServiceBusPubsub{
				Namespace: "",
				Topics: map[string]*AzureTopic{
					"topic1": {Name: "azure-topic-1"},
				},
			},
			wantErr:  true,
			errField: "namespace",
		},
		{
			name: "no topics is valid",
			pubsub: &AzureServiceBusPubsub{
				Namespace: "my-namespace",
				Topics:    map[string]*AzureTopic{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			_, errs := Validate(tt.pubsub)

			if tt.wantErr {
				c.Assert(len(errs) > 0, qt.IsTrue, qt.Commentf("expected validation errors"))
				if tt.errField != "" {
					found := false
					for path := range errs {
						if path.String() == "."+tt.errField {
							found = true
							break
						}
					}
					c.Assert(found, qt.IsTrue, qt.Commentf("expected error for field %q", tt.errField))
				}
			} else {
				c.Assert(len(errs), qt.Equals, 0, qt.Commentf("unexpected errors: %v", errs))
			}
		})
	}
}

// TestAzureServiceBusPubsub_DeleteTopic tests deleting topics from Azure Service Bus.
func TestAzureServiceBusPubsub_DeleteTopic(t *testing.T) {
	c := qt.New(t)

	pubsub := &AzureServiceBusPubsub{
		Namespace: "my-namespace",
		Topics: map[string]*AzureTopic{
			"topic1": {Name: "azure-topic-1"},
			"topic2": {Name: "azure-topic-2"},
		},
	}

	// Delete existing topic
	pubsub.DeleteTopic("topic1")
	c.Assert(len(pubsub.Topics), qt.Equals, 1)
	_, exists := pubsub.Topics["topic1"]
	c.Assert(exists, qt.IsFalse)
	_, exists = pubsub.Topics["topic2"]
	c.Assert(exists, qt.IsTrue)

	// Delete non-existent topic (should be no-op)
	pubsub.DeleteTopic("nonexistent")
	c.Assert(len(pubsub.Topics), qt.Equals, 1)
}

// TestAzureTopic_Validate tests validation of Azure Topic configurations.
func TestAzureTopic_Validate(t *testing.T) {
	tests := []struct {
		name     string
		topic    *AzureTopic
		wantErr  bool
		errField string
	}{
		{
			name: "valid config with subscriptions",
			topic: &AzureTopic{
				Name: "my-topic",
				Subscriptions: map[string]*AzureSub{
					"sub1": {Name: "my-subscription"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config without subscriptions",
			topic: &AzureTopic{
				Name:          "my-topic",
				Subscriptions: map[string]*AzureSub{},
			},
			wantErr: false,
		},
		{
			name: "empty topic name",
			topic: &AzureTopic{
				Name: "",
				Subscriptions: map[string]*AzureSub{
					"sub1": {Name: "my-subscription"},
				},
			},
			wantErr:  true,
			errField: "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			_, errs := Validate(tt.topic)

			if tt.wantErr {
				c.Assert(len(errs) > 0, qt.IsTrue, qt.Commentf("expected validation errors"))
				if tt.errField != "" {
					found := false
					for path := range errs {
						if path.String() == "."+tt.errField {
							found = true
							break
						}
					}
					c.Assert(found, qt.IsTrue, qt.Commentf("expected error for field %q", tt.errField))
				}
			} else {
				c.Assert(len(errs), qt.Equals, 0, qt.Commentf("unexpected errors: %v", errs))
			}
		})
	}
}

// TestAzureTopic_DeleteSubscription tests deleting subscriptions from Azure Topic.
func TestAzureTopic_DeleteSubscription(t *testing.T) {
	c := qt.New(t)

	topic := &AzureTopic{
		Name: "my-topic",
		Subscriptions: map[string]*AzureSub{
			"sub1": {Name: "azure-sub-1"},
			"sub2": {Name: "azure-sub-2"},
		},
	}

	// Delete existing subscription
	topic.DeleteSubscription("sub1")
	c.Assert(len(topic.Subscriptions), qt.Equals, 1)
	_, exists := topic.Subscriptions["sub1"]
	c.Assert(exists, qt.IsFalse)
	_, exists = topic.Subscriptions["sub2"]
	c.Assert(exists, qt.IsTrue)

	// Delete non-existent subscription (should be no-op)
	topic.DeleteSubscription("nonexistent")
	c.Assert(len(topic.Subscriptions), qt.Equals, 1)
}

// TestAzureSub_Validate tests validation of Azure Subscription configurations.
func TestAzureSub_Validate(t *testing.T) {
	tests := []struct {
		name     string
		sub      *AzureSub
		wantErr  bool
		errField string
	}{
		{
			name: "valid config",
			sub: &AzureSub{
				Name: "my-subscription",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			sub: &AzureSub{
				Name: "",
			},
			wantErr:  true,
			errField: "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			_, errs := Validate(tt.sub)

			if tt.wantErr {
				c.Assert(len(errs) > 0, qt.IsTrue, qt.Commentf("expected validation errors"))
				if tt.errField != "" {
					found := false
					for path := range errs {
						if path.String() == "."+tt.errField {
							found = true
							break
						}
					}
					c.Assert(found, qt.IsTrue, qt.Commentf("expected error for field %q", tt.errField))
				}
			} else {
				c.Assert(len(errs), qt.Equals, 0, qt.Commentf("unexpected errors: %v", errs))
			}
		})
	}
}

// TestAzureMonitor_Validate tests validation of Azure Monitor configurations.
func TestAzureMonitor_Validate(t *testing.T) {
	tests := []struct {
		name     string
		monitor  *AzureMonitor
		wantErr  bool
		errField string
	}{
		{
			name: "valid config",
			monitor: &AzureMonitor{
				Location:          "eastus",
				SubscriptionID:    "sub-12345",
				ResourceGroup:     "my-rg",
				ResourceNamespace: "Microsoft.ContainerApps",
				ResourceName:      "my-app",
				Namespace:         "my-namespace",
			},
			wantErr: false,
		},
		{
			name: "missing location",
			monitor: &AzureMonitor{
				Location:          "",
				SubscriptionID:    "sub-12345",
				ResourceGroup:     "my-rg",
				ResourceNamespace: "Microsoft.ContainerApps",
				ResourceName:      "my-app",
				Namespace:         "my-namespace",
			},
			wantErr:  true,
			errField: "location",
		},
		{
			name: "missing subscription_id",
			monitor: &AzureMonitor{
				Location:          "eastus",
				SubscriptionID:    "",
				ResourceGroup:     "my-rg",
				ResourceNamespace: "Microsoft.ContainerApps",
				ResourceName:      "my-app",
				Namespace:         "my-namespace",
			},
			wantErr:  true,
			errField: "subscription_id",
		},
		{
			name: "missing resource_group",
			monitor: &AzureMonitor{
				Location:          "eastus",
				SubscriptionID:    "sub-12345",
				ResourceGroup:     "",
				ResourceNamespace: "Microsoft.ContainerApps",
				ResourceName:      "my-app",
				Namespace:         "my-namespace",
			},
			wantErr:  true,
			errField: "resource_group",
		},
		{
			name: "missing resource_namespace",
			monitor: &AzureMonitor{
				Location:          "eastus",
				SubscriptionID:    "sub-12345",
				ResourceGroup:     "my-rg",
				ResourceNamespace: "",
				ResourceName:      "my-app",
				Namespace:         "my-namespace",
			},
			wantErr:  true,
			errField: "resource_namespace",
		},
		{
			name: "missing resource_name",
			monitor: &AzureMonitor{
				Location:          "eastus",
				SubscriptionID:    "sub-12345",
				ResourceGroup:     "my-rg",
				ResourceNamespace: "Microsoft.ContainerApps",
				ResourceName:      "",
				Namespace:         "my-namespace",
			},
			wantErr:  true,
			errField: "resource_name",
		},
		{
			name: "missing namespace",
			monitor: &AzureMonitor{
				Location:          "eastus",
				SubscriptionID:    "sub-12345",
				ResourceGroup:     "my-rg",
				ResourceNamespace: "Microsoft.ContainerApps",
				ResourceName:      "my-app",
				Namespace:         "",
			},
			wantErr:  true,
			errField: "namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			_, errs := Validate(tt.monitor)

			if tt.wantErr {
				c.Assert(len(errs) > 0, qt.IsTrue, qt.Commentf("expected validation errors"))
				if tt.errField != "" {
					found := false
					for path := range errs {
						if path.String() == "."+tt.errField {
							found = true
							break
						}
					}
					c.Assert(found, qt.IsTrue, qt.Commentf("expected error for field %q", tt.errField))
				}
			} else {
				c.Assert(len(errs), qt.Equals, 0, qt.Commentf("unexpected errors: %v", errs))
			}
		})
	}
}

// TestAzureServiceBusPubsub_GetTopics tests retrieving topics map.
func TestAzureServiceBusPubsub_GetTopics(t *testing.T) {
	c := qt.New(t)

	pubsub := &AzureServiceBusPubsub{
		Namespace: "my-namespace",
		Topics: map[string]*AzureTopic{
			"topic1": {Name: "azure-topic-1"},
			"topic2": {Name: "azure-topic-2"},
		},
	}

	topics := pubsub.GetTopics()
	c.Assert(len(topics), qt.Equals, 2)
	c.Assert(topics["topic1"], qt.Not(qt.IsNil))
	c.Assert(topics["topic2"], qt.Not(qt.IsNil))
}

// TestAzureTopic_GetSubscriptions tests retrieving subscriptions map.
func TestAzureTopic_GetSubscriptions(t *testing.T) {
	c := qt.New(t)

	topic := &AzureTopic{
		Name: "my-topic",
		Subscriptions: map[string]*AzureSub{
			"sub1": {Name: "azure-sub-1"},
			"sub2": {Name: "azure-sub-2"},
		},
	}

	subs := topic.GetSubscriptions()
	c.Assert(len(subs), qt.Equals, 2)
	c.Assert(subs["sub1"], qt.Not(qt.IsNil))
	c.Assert(subs["sub2"], qt.Not(qt.IsNil))
}
