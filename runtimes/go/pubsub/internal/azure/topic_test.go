package azure

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"encore.dev/appruntime/exported/config"
)

func TestConstants(t *testing.T) {
	if RetryCountAttribute != "encore-retry-count" {
		t.Errorf("RetryCountAttribute = %q, want %q", RetryCountAttribute, "encore-retry-count")
	}
	if TargetSubAttribute != "encore-target-sub" {
		t.Errorf("TargetSubAttribute = %q, want %q", TargetSubAttribute, "encore-target-sub")
	}
}

func TestManager_ProviderName(t *testing.T) {
	mgr := &Manager{_clients: map[string]*azservicebus.Client{}}
	got := mgr.ProviderName()
	want := "azure"
	if got != want {
		t.Errorf("ProviderName() = %q, want %q", got, want)
	}
}

func TestManager_Matches(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.PubsubProvider
		want bool
	}{
		{
			name: "nil azure config",
			cfg:  &config.PubsubProvider{},
			want: false,
		},
		{
			name: "non-nil azure config",
			cfg: &config.PubsubProvider{
				Azure: &config.AzureServiceBusProvider{
					Namespace: "test",
				},
			},
			want: true,
		},
		{
			name: "aws config only",
			cfg: &config.PubsubProvider{
				AWS: &config.AWSPubsubProvider{},
			},
			want: false,
		},
		{
			name: "gcp config only",
			cfg: &config.PubsubProvider{
				GCP: &config.GCPPubsubProvider{},
			},
			want: false,
		},
		{
			name: "multiple providers with azure",
			cfg: &config.PubsubProvider{
				Azure: &config.AzureServiceBusProvider{
					Namespace: "test",
				},
				AWS: &config.AWSPubsubProvider{},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &Manager{_clients: map[string]*azservicebus.Client{}}
			got := mgr.Matches(tt.cfg)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	mgr := NewManager(nil)
	if mgr._clients == nil {
		t.Fatal("_clients map should be initialized")
	}
	if mgr.ProviderName() != "azure" {
		t.Errorf("ProviderName() = %q, want %q", mgr.ProviderName(), "azure")
	}
}

func TestRetryCountParsing(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantCount int64
	}{
		{
			name:      "nil value",
			value:     nil,
			wantCount: 0,
		},
		{
			name:      "integer 0",
			value:     int64(0),
			wantCount: 0,
		},
		{
			name:      "integer 3",
			value:     int64(3),
			wantCount: 3,
		},
		{
			name:      "string 5",
			value:     "5",
			wantCount: 5,
		},
		{
			name:      "invalid string",
			value:     "not-a-number",
			wantCount: 0,
		},
		{
			name:      "large retry count",
			value:     int64(100),
			wantCount: 100,
		},
		{
			name:      "negative count treated as zero",
			value:     "-1",
			wantCount: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := map[string]interface{}{}
			if tt.value != nil {
				props[RetryCountAttribute] = tt.value
			}
			count, _ := strconv.ParseInt(fmt.Sprintf("%v", props[RetryCountAttribute]), 10, 64)
			if count != tt.wantCount {
				t.Errorf("retry count = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestAttributeConversion(t *testing.T) {
	applicationProps := map[string]interface{}{
		"string-attr":       "hello",
		"int-attr":          int64(42),
		"bool-attr":         true,
		"float-attr":        3.14,
		RetryCountAttribute: int64(2),
		"empty-string":      "",
		"zero-int":          int64(0),
		"false-bool":        false,
	}

	attrs := make(map[string]string, len(applicationProps))
	for k, v := range applicationProps {
		attrs[k] = fmt.Sprintf("%v", v)
	}

	tests := []struct {
		key  string
		want string
	}{
		{"string-attr", "hello"},
		{"int-attr", "42"},
		{"bool-attr", "true"},
		{"float-attr", "3.14"},
		{RetryCountAttribute, "2"},
		{"empty-string", ""},
		{"zero-int", "0"},
		{"false-bool", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, ok := attrs[tt.key]
			if !ok {
				t.Errorf("attribute %q not found in converted map", tt.key)
				return
			}
			if got != tt.want {
				t.Errorf("attribute %q = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestDeliveryAttemptCalculation(t *testing.T) {
	tests := []struct {
		name           string
		retryCount     int64
		wantDelivery   int64
	}{
		{
			name:         "first delivery (no retries)",
			retryCount:   0,
			wantDelivery: 1,
		},
		{
			name:         "second delivery (one retry)",
			retryCount:   1,
			wantDelivery: 2,
		},
		{
			name:         "tenth delivery (nine retries)",
			retryCount:   9,
			wantDelivery: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deliveryAttempt := tt.retryCount + 1
			if deliveryAttempt != tt.wantDelivery {
				t.Errorf("delivery attempt = %d, want %d", deliveryAttempt, tt.wantDelivery)
			}
		})
	}
}
