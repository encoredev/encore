package run

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/rs/xid"

	encore "encore.dev"
	"encore.dev/appruntime/config"
	"encr.dev/compiler"
	"encr.dev/internal/env"
	"encr.dev/internal/version"
	"encr.dev/parser"
	"encr.dev/pkg/cueutil"
	"encr.dev/v2/cli/daemon/apps"
	"encr.dev/v2/cli/daemon/localinfra/secret"
	"encr.dev/v2/cli/daemon/localinfra/sqldb"
)

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

	// Secrets are the secrets to use.
	Secrets *secret.LoadResult

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
	expSet, err := params.App.Experiments(params.Environ)
	if err != nil {
		return err
	}

	secretData, err := params.Secrets.Get(ctx, expSet)
	if err != nil {
		return err
	}
	secrets := secretData.Values

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
		Parse:                 params.Parse,
		Revision:              params.Parse.Meta.AppRevision,
		UncommittedChanges:    params.Parse.Meta.UncommittedChanges,
		WorkingDir:            params.WorkingDir,
		CgoEnabled:            true,
		EncoreCompilerVersion: fmt.Sprintf("EncoreCLI/%s", version.Version),
		EncoreRuntimePath:     env.EncoreRuntimePath(),
		EncoreGoRoot:          env.EncoreGoRoot(),
		BuildTags:             []string{"encore_local", "encore_no_gcp", "encore_no_aws", "encore_no_azure"},
		Experiments:           expSet,
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
