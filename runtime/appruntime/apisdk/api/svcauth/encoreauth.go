package svcauth

import (
	"fmt"
	"sort"
	"time"

	"github.com/benbjohnson/clock"
	"go.encore.dev/platform-sdk/pkg/auth"
	"golang.org/x/crypto/sha3"

	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/exported/config"
	"encore.dev/beta/errs"
)

const ecAuthHashHeader = "Svc-Auth"
const ecDateHeader = "Date"

// encoreAuth is a ServiceAuth implementation that uses the Encore auth package to sign requests.
type encoreAuth struct {
	appSlug   string
	envName   string
	keys      []auth.Key
	latestKey auth.Key
	clock     clock.Clock
}

func newEncoreAuth(clock clock.Clock, appSlug string, envName string, keys []config.EncoreAuthKey) ServiceAuth {
	var keySet []auth.Key
	var latestKey auth.Key
	for _, key := range keys {
		keySet = append(keySet, auth.Key{
			KeyID: key.KeyID,
			Data:  key.Data,
		})
		if latestKey.KeyID < key.KeyID {
			latestKey = auth.Key{
				KeyID: key.KeyID,
				Data:  key.Data,
			}
		}
	}

	return &encoreAuth{
		appSlug:   appSlug,
		envName:   envName,
		keys:      keySet,
		latestKey: latestKey,
		clock:     clock,
	}
}

func (ea *encoreAuth) method() string {
	return "encore-auth"
}

func (ea *encoreAuth) verify(req transport.Transport) error {
	headers := &auth.Headers{}
	if authStr, found := req.ReadMeta(ecAuthHashHeader); !found {
		return auth.ErrNoAuthorizationHeader
	} else {
		headers.Authorization = authStr
	}
	if dateStr, found := req.ReadMeta(ecDateHeader); !found {
		return auth.ErrNoDateHeader
	} else {
		headers.Date = dateStr
	}

	keyID, appSlug, envName, timestamp, opHash, err := headers.SigningComponents()
	if err != nil {
		return err
	}

	// First the timestamp, and don't do any work if it's too old or too new
	const allowedClockSkew = 2 * time.Minute
	if diff := ea.clock.Since(timestamp); diff > allowedClockSkew || diff < -allowedClockSkew {
		return auth.ErrAuthenticationExpired
	}

	// Find the key
	var key auth.Key
	for _, k := range ea.keys {
		if k.KeyID == keyID {
			key = k
			break
		}
	}
	if key.KeyID == 0 {
		return auth.ErrAuthenticationFailed
	}

	// Rebuild the signature
	expectedHeaders := auth.SignForVerification(&key, appSlug, envName, timestamp, opHash)

	// Verify the signature
	if !expectedHeaders.Equal(headers) {
		return auth.ErrAuthenticationFailed
	}

	// Now we're verified the signature - now let's compare the OpHash received
	// against the OpHash we would have generated for this request.
	// We do this here to minimize the risk of timing attacks.
	expectedOpHash, err := ea.buildOpHash(req)
	if err != nil {
		return err
	}
	if expectedOpHash != opHash {
		return auth.ErrAuthenticationFailed
	}

	return nil
}

func (ea *encoreAuth) sign(req transport.Transport) error {
	opHash, err := ea.buildOpHash(req)
	if err != nil {
		return err
	}

	headers := auth.Sign(&ea.latestKey, ea.appSlug, ea.envName, ea.clock, opHash)

	req.SetMeta(ecAuthHashHeader, headers.Authorization)
	req.SetMeta(ecDateHeader, headers.Date)
	return nil
}

// buildOpHash builds the operation hash for the request.
func (ea *encoreAuth) buildOpHash(req transport.Transport) (auth.OperationHash, error) {
	// Build a deterministic hash of the meta keys and values
	hash := sha3.New256()
	for _, key := range req.ListMetaKeys() {
		switch key {
		case AuthMethodMetaKey, ecAuthHashHeader, ecDateHeader:
			// Skip these headers, as they are part of the auth mechanism itself

		default:
			// Read all values for this key, and sort them
			values, found := req.ReadMetaValues(key)
			if !found {
				return "", errs.B().Code(errs.Internal).Msg("failed to read metadata value").Err()
			}
			sort.Strings(values)

			for _, value := range values {
				if _, err := fmt.Fprintf(hash, "%s=%s\n", key, value); err != nil {
					return "", errs.B().Code(errs.Internal).Cause(err).Msg("failed to write to hash").Err()
				}
			}
		}
	}

	// Generate the operation hash
	opHash, err := auth.NewOperationHash("internal-api", "call", auth.BytesPayload(hash.Sum(nil)))
	if err != nil {
		return "", errs.B().Code(errs.Internal).Cause(err).Msg("failed to create operation hash for internal API call").Err()
	}
	return opHash, nil
}
