package run

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strconv"

	"encr.dev/cli/daemon/runtime/config"
	"encr.dev/cli/internal/appfile"
	"encr.dev/cli/internal/env"
	"encr.dev/compiler"
	"encr.dev/parser"
)

// Check checks the app for errors.
// It reports a buildDir (if available) when codegenDebug is true.
func (mgr *Manager) Check(ctx context.Context, appRoot, relwd string, codegenDebug bool) (buildDir string, err error) {
	// TODO: We should check that all secret keys are defined as well.
	cfg := &compiler.Config{
		Version:           "", // not needed until we start storing trace metadata
		WorkingDir:        relwd,
		CgoEnabled:        true,
		EncoreRuntimePath: env.EncoreRuntimePath(),
		EncoreGoRoot:      env.EncoreGoRoot(),
		KeepOutput:        codegenDebug,
	}
	result, err := compiler.Build(appRoot, cfg)
	if result != nil && result.Dir != "" {
		if codegenDebug {
			buildDir = result.Dir
		} else {
			os.RemoveAll(result.Dir)
		}
	}
	return buildDir, err
}

// TestParams groups the parameters for the Test method.
type TestParams struct {
	// AppRoot is the application root.
	AppRoot string

	// AppID is the unique app id, as defined by the manifest.
	AppID string

	// WorkingDir is the working dir, for formatting
	// error messages with relative paths.
	WorkingDir string

	// DBClusterID is the database cluster id to connect to.
	DBClusterID string

	// Parse is the parse result for the initial run of the app.
	// It must be set.
	Parse *parser.Result

	// Args are the arguments to pass to "go test".
	Args []string

	// Environ are the environment variables to set when running the tests,
	// in the same format as os.Environ().
	Environ []string

	// Stdout and Stderr are where "go test" output should be written.
	Stdout, Stderr io.Writer
}

// Test runs the tests.
func (mgr *Manager) Test(ctx context.Context, params TestParams) (err error) {
	appSlug, err := appfile.Slug(params.AppRoot)
	if err != nil {
		return err
	}

	var secrets map[string]string
	if appSlug != "" {
		data, err := mgr.Secret.Get(ctx, appSlug)
		if err != nil {
			return err
		}
		secrets = data.Values
	}

	var dbs []*config.SQLDatabase
	for _, svc := range params.Parse.Meta.Svcs {
		if len(svc.Migrations) > 0 {
			dbs = append(dbs, &config.SQLDatabase{
				EncoreName:   svc.Name,
				DatabaseName: svc.Name,
				Host:         "localhost:" + strconv.Itoa(mgr.DBProxyPort),
				User:         "encore",
				Password:     params.DBClusterID,
			})
		}
	}
	runtimeJSON, err := json.Marshal(&config.Runtime{
		AppID:         "test",
		EnvID:         "test",
		EnvName:       "local",
		TraceEndpoint: "http://localhost:" + strconv.Itoa(mgr.RuntimePort) + "/trace",
		SQLDatabases:  dbs,
	})
	if err != nil {
		return err
	}

	cfg := &compiler.Config{
		Version:           "", // not needed until we start storing trace metadata
		WorkingDir:        params.WorkingDir,
		CgoEnabled:        true,
		EncoreRuntimePath: env.EncoreRuntimePath(),
		EncoreGoRoot:      env.EncoreGoRoot(),
		Test: &compiler.TestConfig{
			Env: append(params.Environ,
				"ENCORE_RUNTIME_CONFIG="+base64.RawURLEncoding.EncodeToString(runtimeJSON),
				"ENCORE_APP_SECRETS="+encodeSecretsEnv(secrets),
			),
			Args:   params.Args,
			Stdout: params.Stdout,
			Stderr: params.Stderr,
		},
	}
	return compiler.Test(ctx, params.AppRoot, cfg)
}
