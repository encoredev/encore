// Package manifest reads and writes Encore app manifests.
package manifest

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Manifest represents the persisted manifest for
// an Encore application. It is not intended to be committed to
// source control.
type Manifest struct {
	// AppID is a unique identifier for the app.
	// It uses the encore.dev app slug if the app
	// is linked, and is otherwise a randomly generated id.
	AppID string `json:"appID,omitempty"`

	// LocalID is a unique id for the app that's only used locally.
	// It is randomly generated on first use.
	LocalID string `json:"local_id"`
}

// ReadOrCreate reads the manifest for the app rooted at appRoot.
// If it doesn't exist it creates it first.
func ReadOrCreate(appRoot string) (mf *Manifest, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("read/create manifest: %v", err)
		}
	}()

	var man Manifest

	// Use the existing manifest if we have one.
	cfgPath := filepath.Join(appRoot, ".encore", "manifest.json")
	if data, err := os.ReadFile(cfgPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	} else if err == nil {
		err = json.Unmarshal(data, &man)
		if err != nil {
			return nil, err
		}
	}

	// Generate a local ID if we don't have one.
	if man.LocalID == "" {
		// If we have a legacy AppID, migrate that over to the local id.
		if man.AppID != "" {
			man.LocalID = man.AppID
			man.AppID = ""
		} else {
			id, err := genID()
			if err != nil {
				return nil, err
			}
			man.LocalID = id
		}
	}

	// Write it back.
	out, _ := json.Marshal(&man)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return nil, err
	} else if err := os.WriteFile(cfgPath, out, 0644); err != nil {
		return nil, err
	}
	return &man, nil
}

const encodeStr = "23456789abcdefghikmnopqrstuvwxyz"

var encoding = base32.NewEncoding(encodeStr).WithPadding(base32.NoPadding)

// genID generates a random id for a local ID
//
// Note: the fact this generates without a hyphen is expected and used
// to identify a local ID vs a platform ID
func genID() (string, error) {
	var data [3]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return encoding.EncodeToString(data[:]), nil
}
