package ecauth

import (
	"crypto/hmac"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/sha3"
)

const (
	signatureVersion = "ENCORE1"
	hashImpl         = "HMAC-SHA3-256"
	authScheme       = signatureVersion + "-" + hashImpl
)

// Sign creates the authorization headers for a new request.
//
// The signature algorithm is based on the AWS Signature Version 4 signing process and is valid for 2 minutes
// from the time the request is signed.
func Sign(key *Key, appSlug, envName string, operation OperationHash) *Headers {
	return SignForVerification(key, appSlug, envName, time.Now(), operation)
}

// SignForVerification uses the [Headers.SigningComponents] from a received request to generate a
// new set of headers that can be used to verify the request using [Headers.Equal].
//
// This function should not be used to sign a new request, for that use [Sign].
func SignForVerification(key *Key, appSlug, envName string, timestamp time.Time, operation OperationHash) *Headers {
	// Build the components of the authorization header
	credentials := createCredentialString(timestamp, appSlug, envName, key)
	requestDigest := buildRequestDigest(timestamp, credentials, operation)
	signingKey := deriveSigningKey(key, timestamp, appSlug, envName)
	signature := hex.EncodeToString(hashHmac(signingKey, []byte(requestDigest)))

	authParameters := strings.Join([]string{
		"cred=" + strconv.Quote(credentials),
		"op=" + operation.HashString(),
		"sig=" + signature,
	}, ", ")

	return &Headers{
		Authorization: authScheme + " " + authParameters,
		Date:          timestamp.UTC().Format(http.TimeFormat),
	}
}

// The credential string is comprised of the current date, app slug, environment name, and key ID
// seperated by slashes.
//
// It is used to as part of the request digest and passed in plaintext to the server.
// This allows the server to verify the request digest was for the correct app and environment.
func createCredentialString(now time.Time, appSlug, envName string, key *Key) string {
	return fmt.Sprintf("%s/%s/%s/%d", now.UTC().Format("20060102"), appSlug, envName, key.KeyID)
}

// The request digest represents the request that we want to make
// and is the data we will sign.
//
// It is a newline separated string of the following:
//
// - The auth scheme being used
// - Timestamp in RFC3339 format
// - App slug and environment name
// - The operation hash
func buildRequestDigest(timestamp time.Time, credentials string, operation OperationHash) string {
	return strings.Join([]string{
		authScheme,
		timestamp.UTC().Format(time.RFC3339),
		credentials,
		operation.HashString(),
	}, "\n")
}

// The signing key is a HMAC-SHA3-256 hash of the following, where each component is hashed in order,
// and the result of each hash is used as the key for the next hash:
//
// - Signature version
// - The shared secret between the app and Encore
// - The date in YYYYMMDD format
// - The application slug
// - The environment name
// - The string "encore_request"
func deriveSigningKey(key *Key, timestamp time.Time, appSlug, envName string) []byte {
	baseKey := append([]byte(signatureVersion), key.Data...)
	dateKey := hashHmac(baseKey, []byte(timestamp.UTC().Format("20060102")))
	appKey := hashHmac(dateKey, []byte(appSlug))
	envKey := hashHmac(appKey, []byte(envName))
	finalKey := hashHmac(envKey, []byte("encore_request"))
	return finalKey
}

func hashHmac(key, data []byte) []byte {
	hash := hmac.New(sha3.New256, key)
	hash.Write(data)
	return hash.Sum(nil)
}
