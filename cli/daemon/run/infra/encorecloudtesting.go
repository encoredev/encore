package infra

import (
	"encoding/base64"
	"strconv"

	"github.com/cockroachdb/errors"

	"go.encore.dev/platform-sdk/pkg/auth"

	"encore.dev/appruntime/exported/config"
)

// setTestEncoreCloud sets the Encore Cloud API configuration to use a local
// Encore Cloud API server.
//
// It returns true if one has been configured, or false if not.
//
// To use it the `encore run` command must be started with the following environment variables:
// - ENCORECLOUD_LOCAL_SERVER: the URL of the local Encore Cloud API server
// - ENCORECLOUD_LOCAL_KEY_ID: the ID of the key to use for authentication
// - ENCORECLOUD_LOCAL_KEY_DATA: the base64-encoded data of the key to use for authentication
func (rm *ResourceManager) setTestEncoreCloud(cfg *config.Runtime) (useLocalCloudServer bool, err error) {
	localServer := rm.environ.Get("ENCORECLOUD_LOCAL_SERVER")
	if localServer == "" {
		return false, nil
	}

	// Get the key and secret
	keyIDStr := rm.environ.Get("ENCORECLOUD_LOCAL_KEY_ID")
	keyData64 := rm.environ.Get("ENCORECLOUD_LOCAL_KEY_DATA")
	if keyIDStr == "" || keyData64 == "" {
		return false, errors.New("ENCORECLOUD_LOCAL_KEY_ID and ENCORECLOUD_LOCAL_KEY_DATA must be set if using ENCORECLOUD_LOCAL_SERVER")
	}

	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil || keyID <= 0 {
		return false, errors.New("ENCORECLOUD_LOCAL_KEY_ID must be a positive integer")
	}

	keyData, err := base64.StdEncoding.DecodeString(keyData64)
	if err != nil {
		return false, errors.New("ENCORECLOUD_LOCAL_KEY_DATA must be a valid base64 string")
	}

	cfg.EncoreCloudAPI = &config.EncoreCloudAPI{
		Server: localServer,
		AuthKeys: []auth.Key{
			{
				KeyID: uint32(keyID),
				Data:  keyData,
			},
		},
	}
	return true, nil
}
