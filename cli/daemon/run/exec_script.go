package run

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"encr.dev/cli/daemon/apps"
	"encr.dev/compiler"
	"encr.dev/internal/env"
	"encr.dev/internal/optracker"
	"encr.dev/internal/version"
	"encr.dev/parser"
	"encr.dev/pkg/cueutil"
)

// ExecScriptParams groups the parameters for the ExecScript method.
type ExecScriptParams struct {
	// App is the app to execute the script for.
	App *apps.Instance

	// ScriptRelPath is the path holding the command. It's either a directory or a files.
	ScriptRelPath string

	// ScriptArgs are the arguments to pass to the script binary.
	ScriptArgs []string

	// WorkingDir is the working dir to execute the script from.
	// It's relative to the app root.
	WorkingDir string

	// Parse is the parse result for the initial run of the app.
	// It must be set.
	Parse *parser.Result

	// Environ are the environment variables to set when running the tests,
	// in the same format as os.Environ().
	Environ []string

	// Stdout and Stderr are where "go test" output should be written.
	Stdout, Stderr io.Writer

	OpTracker *optracker.OpTracker
}

// ExecScript executes the script.
func (mgr *Manager) ExecScript(ctx context.Context, p ExecScriptParams) (err error) {
	expSet, err := p.App.Experiments(p.Environ)
	if err != nil {
		return err
	}

	rs := NewResourceServices(p.App, mgr.ClusterMgr)
	defer rs.StopAll()

	tracker := p.OpTracker
	jobs := NewAsyncBuildJobs(ctx, p.App.PlatformOrLocalID(), tracker)

	// Parse the app source code
	// Parse the app to figure out what infrastructure is needed.
	start := time.Now()
	parseOp := tracker.Add("Building Encore application graph", start)
	topoOp := tracker.Add("Analyzing service topology", start)
	parse, err := mgr.parseApp(parseAppParams{
		App:        p.App,
		Environ:    p.Environ,
		WorkingDir: p.WorkingDir,
		ParseTests: false,
	})
	if err != nil {
		tracker.Fail(parseOp, err)
		return err
	}
	tracker.Done(parseOp, 500*time.Millisecond)
	tracker.Done(topoOp, 300*time.Millisecond)

	if err := rs.StartRequiredServices(jobs, parse); err != nil {
		return err
	}

	var secrets map[string]string
	if usesSecrets(parse.Meta) {
		jobs.Go("Fetching application secrets", true, 150*time.Millisecond, func(ctx context.Context) error {
			if p.App.PlatformID() == "" {
				return fmt.Errorf("the app defines secrets, but is not yet linked to encore.dev; link it with `encore app link` to use secrets")
			}
			data, err := mgr.Secret.Get(ctx, p.App, expSet)
			if err != nil {
				return err
			}
			secrets = data.Values
			return nil
		})
	}

	apiBaseURL := fmt.Sprintf("http://localhost:%d", mgr.RuntimePort)

	var build *compiler.Result
	jobs.Go("Compiling application source code", false, 0, func(ctx context.Context) (err error) {
		cfg := &compiler.Config{
			Parse:                 p.Parse,
			Revision:              p.Parse.Meta.AppRevision,
			UncommittedChanges:    p.Parse.Meta.UncommittedChanges,
			WorkingDir:            p.WorkingDir,
			CgoEnabled:            true,
			EncoreCompilerVersion: fmt.Sprintf("EncoreCLI/%s", version.Version),
			EncoreRuntimePath:     env.EncoreRuntimePath(),
			EncoreGoRoot:          env.EncoreGoRoot(),
			BuildTags:             []string{"encore_local", "encore_no_gcp", "encore_no_aws", "encore_no_azure"},
			Experiments:           expSet,
			Meta: &cueutil.Meta{
				APIBaseURL: apiBaseURL,
				EnvName:    "local",
				EnvType:    cueutil.EnvType_Development,
				CloudType:  cueutil.CloudType_Local,
			},
		}
		build, err = compiler.ExecScript(p.App.Root(), cfg)
		if err != nil {
			return fmt.Errorf("compile error:\n%v", err)
		}
		return nil
	})
	defer func() {
		if err != nil && build != nil {
			os.RemoveAll(build.Dir)
		}
	}()

	if err := jobs.Wait(); err != nil {
		return err
	}

	runtimeCfg := mgr.generateConfig(generateConfigParams{
		App:         p.App,
		RS:          rs,
		Meta:        p.Parse.Meta,
		ForTests:    false,
		AuthKey:     genAuthKey(),
		APIBaseURL:  apiBaseURL,
		ConfigAppID: GenID(),
		ConfigEnvID: GenID(),
	})
	runtimeJSON, _ := json.Marshal(runtimeCfg)

	env := append(os.Environ(), p.Environ...)
	env = append(env,
		"ENCORE_RUNTIME_CONFIG="+base64.RawURLEncoding.EncodeToString(runtimeJSON),
		"ENCORE_APP_SECRETS="+encodeSecretsEnv(secrets),
	)
	for serviceName, cfgString := range build.Configs {
		env = append(env, "ENCORE_CFG_"+strings.ToUpper(serviceName)+"="+base64.RawURLEncoding.EncodeToString([]byte(cfgString)))
	}

	tracker.AllDone()

	cmd := exec.CommandContext(ctx, build.Exe, p.ScriptArgs...)
	cmd.Dir = filepath.Join(p.App.Root(), p.WorkingDir)
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	cmd.Env = env
	return cmd.Run()
}
