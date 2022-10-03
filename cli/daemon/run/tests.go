package run

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/rs/xid"

	encore "encore.dev"
	"encore.dev/appruntime/config"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/env"
	"encr.dev/cli/internal/version"
	"encr.dev/compiler"
	"encr.dev/parser"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/vcs"
)

// Check checks the app for errors.
// It reports a buildDir (if available) when codegenDebug is true.
func (mgr *Manager) Check(ctx context.Context, appRoot, relwd string, codegenDebug bool) (buildDir string, err error) {
	vcsRevision := vcs.GetRevision(appRoot)

	// TODO: We should check that all secret keys are defined as well.
	cfg := &compiler.Config{
		Revision:              vcsRevision.Revision,
		UncommittedChanges:    vcsRevision.Uncommitted,
		WorkingDir:            relwd,
		CgoEnabled:            true,
		EncoreCompilerVersion: fmt.Sprintf("EncoreCLI/%s", version.Version),
		EncoreRuntimePath:     env.EncoreRuntimePath(),
		EncoreGoRoot:          env.EncoreGoRoot(),
		KeepOutput:            codegenDebug,
		BuildTags:             []string{"encore_local"},
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
	// App is the app to test.
	App *apps.Instance

	// SQLDBCluster is the SQLDB cluster to use, if any.
	SQLDBCluster *sqldb.Cluster

	// WorkingDir is the working dir, for formatting
	// error messages with relative paths.
	WorkingDir string

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
	var secrets map[string]string
	if pid := params.App.PlatformID(); pid != "" {
		data, err := mgr.Secret.Get(ctx, pid)
		if err != nil {
			return err
		}
		secrets = data.Values
	}

	var (
		sqlServers []*config.SQLServer
		sqlDBs     []*config.SQLDatabase
	)
	if params.SQLDBCluster != nil {
		srv := &config.SQLServer{
			Host: "localhost:" + strconv.Itoa(mgr.DBProxyPort),
		}
		sqlServers = append(sqlServers, srv)
		for _, svc := range params.Parse.Meta.Svcs {
			if len(svc.Migrations) > 0 {
				sqlDBs = append(sqlDBs, &config.SQLDatabase{
					ServerID:     0,
					EncoreName:   svc.Name,
					DatabaseName: svc.Name,
					User:         "encore",
					Password:     params.SQLDBCluster.Password,
				})
			}
		}

		// Configure max connections based on 96 connections
		// divided evenly among the databases
		maxConns := 96 / len(sqlDBs)
		for _, db := range sqlDBs {
			db.MaxConnections = maxConns
		}
	}

	apiBaseURL := fmt.Sprintf("http://localhost:%d", mgr.RuntimePort)

	runtimeJSON, err := json.Marshal(&config.Runtime{
		AppID:         "test",
		AppSlug:       params.App.PlatformID(),
		APIBaseURL:    apiBaseURL,
		DeployID:      fmt.Sprintf("clitest_%s", xid.New()),
		DeployedAt:    time.Now(),
		EnvID:         "test",
		EnvName:       "local",
		EnvCloud:      string(encore.CloudLocal),
		EnvType:       string(encore.EnvTest),
		TraceEndpoint: "http://localhost:" + strconv.Itoa(mgr.RuntimePort) + "/trace",
		SQLDatabases:  sqlDBs,
		SQLServers:    sqlServers,
		AuthKeys:      []config.EncoreAuthKey{genAuthKey()},
	})
	if err != nil {
		return err
	}

	cfg := &compiler.Config{
		Revision:              params.Parse.Meta.AppRevision,
		UncommittedChanges:    params.Parse.Meta.UncommittedChanges,
		WorkingDir:            params.WorkingDir,
		CgoEnabled:            true,
		EncoreCompilerVersion: fmt.Sprintf("EncoreCLI/%s", version.Version),
		EncoreRuntimePath:     env.EncoreRuntimePath(),
		EncoreGoRoot:          env.EncoreGoRoot(),
		BuildTags:             []string{"encore_local"},
		Meta: &cueutil.Meta{
			APIBaseURL: apiBaseURL,
			EnvName:    "local",
			EnvType:    cueutil.EnvType_Test,
			CloudType:  cueutil.CloudType_Local,
		},
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
	return compiler.Test(ctx, params.App.Root(), cfg)
}
