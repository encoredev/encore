package run

import (
	"context"
	"io"
	"os"
	"strconv"

	"encr.dev/cli/daemon/internal/appfile"
	"encr.dev/cli/internal/env"
	"encr.dev/compiler"
	"encr.dev/parser"
)

// Check checks the app for errors.
func (mgr *Manager) Check(ctx context.Context, appRoot, relwd string) error {
	// TODO: We should check that all secret keys are defined as well.
	cfg := &compiler.Config{
		Version:           "", // not needed until we start storing trace metadata
		WorkingDir:        relwd,
		CgoEnabled:        true,
		EncoreRuntimePath: env.EncoreRuntimePath(),
		EncoreGoRoot:      env.EncoreGoRoot(),
	}
	result, err := compiler.Build(appRoot, cfg)
	if err == nil {
		os.RemoveAll(result.Dir)
	}
	return err
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
	// If nil the app is parsed before starting.
	Parse *parser.Result

	// Args are the arguments to pass to "go test".
	Args []string

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

	cfg := &compiler.Config{
		Version:           "", // not needed until we start storing trace metadata
		WorkingDir:        params.WorkingDir,
		CgoEnabled:        true,
		EncoreRuntimePath: env.EncoreRuntimePath(),
		EncoreGoRoot:      env.EncoreGoRoot(),
		Test: &compiler.TestConfig{
			Env: []string{
				"ENCORE_ENV_ID=test",
				"ENCORE_PROC_ID=test",
				"ENCORE_RUNTIME_ADDRESS=localhost:" + strconv.Itoa(mgr.RuntimePort),
				"ENCORE_SQLDB_ADDRESS=localhost:" + strconv.Itoa(mgr.DBProxyPort),
				"ENCORE_SQLDB_PASSWORD=" + params.DBClusterID,
				"ENCORE_SECRETS=" + encodeSecretsEnv(secrets),
			},
			Args:   params.Args,
			Stdout: params.Stdout,
			Stderr: params.Stderr,
		},
	}
	return compiler.Test(ctx, params.AppRoot, cfg)
}
