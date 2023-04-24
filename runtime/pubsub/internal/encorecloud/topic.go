package encorecloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/beta/errs"
	"encore.dev/internal/ecauth"
	"encore.dev/pubsub/internal/types"
)

type publishParams struct {
	Attributes map[string]string `json:"attributes,omitempty"` // Optional attributes for this message.
	Payload    json.RawMessage   `json:"payload"`              // The message payload.
}

type publishResponse struct {
	MessageID string `json:"message_id"`
}

type topic struct {
	mgr *Manager
	cfg *config.PubsubTopic
}

func (t *topic) PublishMessage(ctx context.Context, attrs map[string]string, data []byte) (id string, err error) {
	// Build the request body
	topicID := t.cfg.ProviderName
	publishBytes, err := json.Marshal(&publishParams{
		Attributes: attrs,
		Payload:    data,
	})
	if err != nil {
		return "", err
	}

	// Hash the request
	opHash, err := ecauth.NewOperationHash(
		ecauth.PubsubMsg, ecauth.Create, ecauth.BytesPayload(publishBytes), []byte(topicID),
	)
	if err != nil {
		return "", err
	}

	// Sign the hash
	key, err := t.mgr.latestAuthKey()
	if err != nil {
		return "", err
	}
	headers := ecauth.Sign(&key, t.mgr.runtime.AppSlug, t.mgr.runtime.EnvName, opHash)

	// Create the request
	url := fmt.Sprintf("%s/v1/pubsub/%s/publish", t.mgr.runtime.EncoreCloudAPI.Server, topicID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(publishBytes))
	if err != nil {
		return "", err
	}

	// Set the headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", headers.Authorization)
	req.Header.Set("Date", headers.Date)

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", errs.B().Code(errs.Internal).Msgf("encorecloud pubsub publish failed with status %d", resp.StatusCode).Err()
	}

	// Decode the response
	typedResp := &publishResponse{}
	if err := json.NewDecoder(resp.Body).Decode(typedResp); err != nil {
		return "", err
	}

	return typedResp.MessageID, nil
}

func (t *topic) Subscribe(logger *zerolog.Logger, _ time.Duration, _ *types.RetryPolicy, subCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	if subCfg.ID == "" {
		panic("encorecloud pubsub subscriptions must have an ID")
	}

	t.mgr.registerPushEndpoint(logger, subCfg, f)
}
