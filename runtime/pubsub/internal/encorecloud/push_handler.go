package encorecloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/beta/errs"
	"encore.dev/internal/ecauth"
	"encore.dev/pubsub/internal/types"
)

// pushPayload is the payload that Encore Cloud will generate
// when pushing a subscription attempt to a push endpoint
type pushPayload struct {
	Data            []byte            `json:"data"`
	Attributes      map[string]string `json:"attributes"`
	MessageID       string            `json:"messageId"`
	PublishTime     time.Time         `json:"publishTime"`
	DeliveryAttempt int               `json:"deliveryAttempt"`
}

// registerPushEndpoint registers a push endpoint for a subscription from Encore Cloud
//
// Encore Cloud will send a POST request to the endpoint with a JSON encoded [pushPayload] as the body.
// The request will be signed with the latest Encore Cloud auth key for this application.
//
// Once the request is received and verified, the user's subscription function will be called with the decoded
// payload, while simultaneously an event stream will be sent back to Encore Cloud to indicate that the request
// is being processed, with keepalive messages being sent every 5 seconds.
//
// If the subscription function returns an error, the event stream will be closed with the error message.
// If the subscription function returns successfully, the event stream will be closed with a success message.
//
// The Encore Cloud server will wait for a valid end response from the event stream before closing the connection and
// acknowledging the message with the underlying message broker.
//
// If the event stream is closed without a valid end response, the message will be nacked and retried by Encore Cloud.
//
// If the request is closed by Encore Cloud while a subscription function is still running, the context of the function
// will be cancelled, as this means Encore Cloud has failed to receive a keepalive message from the event stream and has
// assumed the request has failed.
//
// The events on the stream will be one of these types:
// - "keepalive"
// - "ack"
// - "nack"
func (mgr *Manager) registerPushEndpoint(logger *zerolog.Logger, subscriptionConfig *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	mgr.pushRegistry.RegisterPushSubscriptionHandler(
		types.SubscriptionID(subscriptionConfig.ID),
		func(w http.ResponseWriter, req *http.Request) {
			payload, err := mgr.decodeAndVerifyPushPayload(req, subscriptionConfig.ID)
			if err != nil {
				logger.Err(err).Msg("error while verifying PubSub subscription message")
				errs.HTTPError(w, err)
				return
			}

			flusher, ok := w.(http.Flusher)
			if !ok {
				err := errs.B().Code(errs.Internal).Msg("unable to cast http.ResponseWriter to http.Flusher").Err()
				logger.Err(err).Msg("error while setting up flushing response")
				errs.HTTPError(w, err)
				return
			}

			// Start the event stream
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			// Run the subscription function in a goroutine
			response := make(chan error)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						err := errs.B().Code(errs.Internal).Msg(fmt.Sprintf("panic while processing PubSub message: %v", r)).Err()
						response <- err
					}
					close(response)
				}()

				response <- f(
					req.Context(),
					payload.MessageID, payload.PublishTime, payload.DeliveryAttempt,
					payload.Attributes, payload.Data,
				)
			}()

			// Wait for the function to complete or the request to be cancelled
			var lastError error
			var finished bool
			keepAliveTimeout := time.NewTicker(5 * time.Second)
			defer keepAliveTimeout.Stop()

			for !finished {
				select {
				case <-req.Context().Done():
					logger.Err(err).Msg("PubSub push endpoint closed by Encore Cloud before subscription function completed")
					return

				case <-keepAliveTimeout.C:
					// Send a keepalive message
					if _, err := fmt.Fprintf(w, "event: keepalive\ndata: \n\n"); err != nil {
						logger.Err(err).Msg("error while sending keepalive message")
					}
					flusher.Flush()

				case err, done := <-response:
					if done {
						finished = true
					} else {
						lastError = err
					}
				}
			}

			// Now that the subscription function has completed, send the end message
			if lastError != nil {
				logger.Err(lastError).Msg("error while handling PubSub subscription message")

				if _, err := fmt.Fprintf(w, "event: nack\ndata: %s\n\n", lastError.Error()); err != nil {
					logger.Err(err).Msg("error while sending nack message")
				}
			} else {
				if _, err := fmt.Fprintf(w, "event: ack\ndata: \n\n"); err != nil {
					logger.Err(err).Msg("error while sending ack message")
				}
			}
			flusher.Flush()

			// Now wait for the request to be closed by Encore Cloud (upto 5 seconds)
			select {
			case <-req.Context().Done():
				// If the request is closed by Encore Cloud, the context will be cancelled, this is a sign that it has processed
				// our end message successfully

			case <-time.After(5 * time.Second):
				// If we get here, the request was not closed by Encore Cloud, so we should log an error
				// and return
				logger.Err(err).Msg("PubSub push connection was not closed by Encore Cloud after ack/nack message sent")
			}
		},
	)
}

// decodeAndVerifyPushPayload decodes the push payload from the request body and verifies the operation hash
// to ensure the request is coming from Encore Cloud and hasn't been tampered with.
func (mgr *Manager) decodeAndVerifyPushPayload(req *http.Request, subscriptionID string) (*pushPayload, error) {
	opHash, err := ecauth.GetVerifiedOperationHash(req, mgr.runtime.EncoreCloudAPI.AuthKeys)
	if err != nil {
		return nil, errs.Wrap(err, "unable to verify operation hash")
	}

	// Body bytes
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, errs.Wrap(err, "unable to read request body")
	}

	// Verify the operation hash is correct
	ok, err := opHash.Verify(
		ecauth.PubsubMsg, ecauth.Read, ecauth.BytesPayload(bodyBytes),
		[]byte(subscriptionID),
	)
	if err != nil {
		return nil, errs.Wrap(err, "unable to verify operation hash")
	}
	if !ok {
		return nil, errs.B().Code(errs.Unauthenticated).Msg("invalid operation hash").Err()
	}

	// Decode the payload
	payload := &pushPayload{}
	if err := json.Unmarshal(bodyBytes, payload); err != nil {
		return nil, errs.WrapCode(err, errs.InvalidArgument, "invalid push payload")
	}

	return payload, nil
}
