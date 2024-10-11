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

	"encore.dev/appruntime/exported/experiments"
)

// Name is the name of the Encore app file.
// It is expected to be located in the root of the Encore app
// (which is usually the Git repository root).
const Name = "encore.app"

type Lang string

const (
	LangGo Lang = "go"
	LangTS Lang = "typescript"
)

// File is a parsed encore.app file.
type File struct {
	// ID is the encore.dev app id for the app.
	// It is empty if the app is not linked to encore.dev.
	ID string `json:"id"` // can be empty

	// Experiments is a list of values to enable experimental features in Encore.
	// These are not guaranteed to be stable in either runtime behaviour
	// or in API design.
	//
	// Do not use these features in production without consulting the Encore team.
	Experiments []experiments.Name `json:"experiments,omitempty"`

	// Lang is the language the app is written in.
	// If empty it defaults to Go.
	Lang Lang `json:"lang"`

	// Configure global CORS settings for the application which
	// will be applied to all API gateways into the application.
	GlobalCORS *CORS `json:"global_cors,omitempty"`

	// Build contains build settings for the application.
	Build Build `json:"build,omitempty"`

	// CgoEnabled enables building with cgo.
	//
	// Deprecated: Use build.cgo_enabled instead.
	CgoEnabled bool `json:"cgo_enabled,omitempty"`

	// DockerBaseImage changes the docker base image used for building the application
	// in Encore's CI/CD system. If unspecified it defaults to "scratch".
	//
	// Deprecated: Use build.docker.base_image instead.
	DockerBaseImage string `json:"docker_base_image,omitempty"`
}

type Build struct {
	// CgoEnabled enables building with cgo.
	CgoEnabled bool `json:"cgo_enabled,omitempty"`

	// Docker configures the docker images built
	// by Encore's CI/CD system.
	Docker Docker `json:"docker,omitempty"`
}

type Docker struct {
	// BaseImage changes the docker base image used for building the application
	// in Encore's CI/CD system. If unspecified it defaults to "scratch".
	BaseImage string `json:"base_image,omitempty"`

	// BundleSource determines whether the source code of the application
	// should be bundled into the binary, at "/workspace".
	BundleSource bool `json:"bundle_source,omitempty"`

	// WorkingDir specifies the working directory to start the docker image in.
	// If empty it defaults to "/workspace" if the source code is bundled, and to "/" otherwise.
	WorkingDir string `json:"working_dir,omitempty"`

	// ProcessPerService specifies whether each service should run in its own process. If false,
	// all services are run in the same process.
	ProcessPerService bool `json:"process_per_service,omitempty"`
}

type CORS struct {
	// Debug enables CORS debug logging.
	Debug bool `json:"debug,omitempty"`

	// AllowHeaders allows an app to specify additional headers that should be
	// accepted by the app.
	//
	// If the list contains "*", then all headers are allowed.
	AllowHeaders []string `json:"allow_headers"`

	// ExposeHeaders allows an app to specify additional headers that should be
	// exposed from the app, beyond the default set always recognized by Encore.
	//
	// If the list contains "*", then all headers are exposed.
	ExposeHeaders []string `json:"expose_headers"`

	// AllowOriginsWithoutCredentials specifies the allowed origins for requests
	// that don't include credentials. If nil it defaults to allowing all domains
	// (equivalent to []string{"*"}).
	AllowOriginsWithoutCredentials []string `json:"allow_origins_without_credentials,omitempty"`

	// AllowOriginsWithCredentials specifies the allowed origins for requests
	// that include credentials. If a request is made from an Origin in this list
	// Encore responds with Access-Control-Allow-Origin: <Origin>.
	//
	// The URLs in this list may include wildcards (e.g. "https://*.example.com"
	// or "https://*-myapp.example.com").
	AllowOriginsWithCredentials []string `json:"allow_origins_with_credentials,omitempty"`
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

	switch f.Lang {
	case LangGo, LangTS:
	// Do nothing
	case "":
		f.Lang = LangGo
	default:
		return nil, fmt.Errorf("appfile.Parse: invalid lang %q", f.Lang)
	}

	// Parse deprecated fields into the new Build struct.
	f.Build.CgoEnabled = f.Build.CgoEnabled || f.CgoEnabled
	if f.Build.Docker.BaseImage == "" {
		f.Build.Docker.BaseImage = f.DockerBaseImage
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

// Experiments returns the experimental feature the app located
// at appRoot has opted into.
func Experiments(appRoot string) ([]experiments.Name, error) {
	f, err := ParseFile(filepath.Join(appRoot, Name))
	if err != nil {
		return nil, err
	}
	return f.Experiments, nil
}

// GlobalCORS returns the global CORS settings for the app located
func GlobalCORS(appRoot string) (*CORS, error) {
	f, err := ParseFile(filepath.Join(appRoot, Name))
	if err != nil {
		return nil, err
	}
	return f.GlobalCORS, nil
}

// AppLang returns the language of the app located at appRoot.
func AppLang(appRoot string) (Lang, error) {
	f, err := ParseFile(filepath.Join(appRoot, Name))
	if err != nil {
		return "", err
	}
	return f.Lang, nil
}
