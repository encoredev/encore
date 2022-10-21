// Package appfile reads and writes encore.app files.
package appfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tailscale/hujson"
)

// Name is the name of the Encore app file.
// It is expected to be located in the root of the Encore app
// (which is usually the Git repository root).
const Name = "encore.app"

// File is a parsed encore.app file.
type File struct {
	// ID is the encore.dev app id for the app.
	// It is empty if the app is not linked to encore.dev.
	ID string `json:"id"` // can be empty

	// Enable experimental features in Encore.
	// These are not guaranteed to be stable in either runtime behaviour
	// or in API design. Do not use these features in production.
	Experiments map[string]bool `json:"experiments,omitempty"`
}

// Parse parses the app file data into a File.
func Parse(data []byte) (*File, error) {
	var f File
	data, err := hujson.Standardize(data)
	if err == nil {
		err = json.Unmarshal(data, &f)
	}
	if err != nil {
		return nil, fmt.Errorf("appfile.Parse: %v", err)
	}
	return &f, nil
}

// ParseFile parses the app file located at path.
func ParseFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &File{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("appfile.ParseFile: %w", err)
	}
	return Parse(data)
}

// Slug parses the app slug for the encore.app file located at path.
// The slug can be empty if the app is not linked to encore.dev.
func Slug(appRoot string) (string, error) {
	f, err := ParseFile(filepath.Join(appRoot, Name))
	if err != nil {
		return "", err
	}
	return f.ID, nil
}

// Experiments returns true if the app has
// opted into experimental features of Encore.
func Experiments(appRoot string) (map[string]bool, error) {
	f, err := ParseFile(filepath.Join(appRoot, Name))
	if err != nil {
		return nil, err
	}
	return f.Experiments, nil
}
