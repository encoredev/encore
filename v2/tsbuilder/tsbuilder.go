package tsbuilder

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/proto"

	"encr.dev/internal/env"
	"encr.dev/internal/version"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	metav1 "encr.dev/proto/encore/parser/meta/v1"
)

func New() *BuilderImpl {
	return &BuilderImpl{
		cmds: make(map[*runningCmd]bool),
	}
}

type BuilderImpl struct {
	mu   sync.Mutex
	cmds map[*runningCmd]bool
}

type parseInput struct {
	AppRoot    string `json:"app_root"`
	PlatformID string `json:"platform_id,omitempty"`
	LocalID    string `json:"local_id"`
	ParseTests bool   `json:"parse_tests"`
}

type data struct {
	cmd    *exec.Cmd
	stdin  io.Writer
	stdout io.Reader
}

func getTSParserPath() (string, error) {
	const tsParserBinaryName = "tsparser-encore"

	if path := os.Getenv("ENCORE_TSPARSER_PATH"); path != "" {
		return path, nil
	}

	// Check the encore bin directory.
	if bin, ok := env.EncoreBin().Get(); ok {
		candidate := filepath.Join(bin, tsParserBinaryName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Now default to the path
	return tsParserBinaryName, nil
}

func (i *BuilderImpl) Close() error {
	i.mu.Lock()
	cmds := maps.Clone(i.cmds)
	i.mu.Unlock()

	for c := range cmds {
		_ = c.cmd.Cancel()
	}

	for c := range cmds {
		<-c.done
	}
	return nil
}

func (i *BuilderImpl) Parse(ctx context.Context, p builder.ParseParams) (*builder.ParseResult, error) {
	exe, err := getTSParserPath()
	if err != nil {
		return nil, err
	}

	runtimesDir := p.Build.EncoreRuntimes.GetOrElseF(func() paths.FS { return paths.FS(env.EncoreRuntimesPath()) })
	jsRuntimePath := jsRuntimeRoot(runtimesDir)

	cmd := exec.CommandContext(ctx, exe)
	cmd.Dir = filepath.Join(p.App.Root(), p.WorkingDir)

	cmd.Env = append(os.Environ(),
		"RUST_LOG=error",
		"RUST_BACKTRACE=1",
	)
	cmd.Env = append(cmd.Env, p.Build.Environ...)
	cmd.Env = append(cmd.Env,
		"ENCORE_JS_RUNTIME_PATH="+jsRuntimePath.ToIO(),
		"ENCORE_APP_REVISION="+p.Build.Revision,
	)

	// If we have an encore-bin directory, add it to the path.
	if bin, ok := env.EncoreBin().Get(); ok {
		cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH")+string(filepath.ListSeparator)+bin)
	}

	// Close the process when the ctx is canceled.
	cmd.WaitDelay = 1 * time.Second

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to get stdin: %s", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to get stdin: %s", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("unable to start builder: %s", err)
	}

	rc := newRunningCmd(cmd)
	i.mu.Lock()
	i.cmds[rc] = true
	i.mu.Unlock()

	go func() {
		<-rc.done
		i.mu.Lock()
		delete(i.cmds, rc)
		i.mu.Unlock()
	}()

	{
		input, _ := json.Marshal(prepareInput{
			AppRoot:         paths.FS(p.App.Root()),
			JSRuntimeRoot:   jsRuntimePath,
			RuntimeVersion:  version.Version,
			UseLocalRuntime: p.Build.UseLocalJSRuntime,
		})
		_, _ = stdin.Write([]byte("prepare\n"))
		if _, err := stdin.Write(input); err != nil {
			return nil, fmt.Errorf("unable to write to stdin: %s", err)
		}

		isSuccess, parseResp, err := readResp(stdout)
		if err != nil {
			return nil, fmt.Errorf("unable to read response: %s", err)
		} else if !isSuccess {
			return nil, errors.New(string(parseResp))
		}
	}

	input, _ := json.Marshal(parseInput{
		AppRoot:    p.App.Root(),
		PlatformID: p.App.PlatformID(),
		LocalID:    p.App.LocalID(),
		ParseTests: p.ParseTests,
	})
	_, _ = stdin.Write([]byte("parse\n"))
	if _, err := stdin.Write(input); err != nil {
		return nil, fmt.Errorf("unable to write to stdin: %s", err)
	}

	isSuccess, parseResp, err := readResp(stdout)
	if err != nil {
		return nil, fmt.Errorf("unable to read response: %s", err)
	} else if !isSuccess {
		return nil, errors.New(string(parseResp))
	}

	var md metav1.Data
	if err := proto.Unmarshal(parseResp, &md); err != nil {
		return nil, fmt.Errorf("unable to parse app: %s", err)
	}

	data := &data{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}

	return &builder.ParseResult{Meta: &md, Data: data}, nil
}

type compileInput struct {
	RuntimeVersion  string            `json:"runtime_version"`
	UseLocalRuntime bool              `json:"use_local_runtime"`
	Debug           builder.DebugMode `json:"debug"`
}

func (i *BuilderImpl) Compile(ctx context.Context, p builder.CompileParams) (*builder.CompileResult, error) {
	data := p.Parse.Data.(*data)

	input, _ := json.Marshal(compileInput{
		RuntimeVersion:  version.Version,
		UseLocalRuntime: p.Build.UseLocalJSRuntime,
		Debug:           p.Build.DebugMode,
	})

	_, _ = data.stdin.Write([]byte("compile\n"))
	if _, err := data.stdin.Write(input); err != nil {
		return nil, fmt.Errorf("unable to write to stdin: %s", err)
	}

	isSuccess, compileResp, err := readResp(data.stdout)
	if err != nil {
		return nil, fmt.Errorf("unable to read response: %s", err)
	} else if !isSuccess {
		return nil, errors.New(string(compileResp))
	}

	var res struct {
		Outputs []*builder.JSBuildOutput `json:"outputs"`
	}

	if err := json.Unmarshal(compileResp, &res); err != nil {
		return nil, fmt.Errorf("unable to decode response: %s", err)
	}

	return &builder.CompileResult{
		OS:   p.Build.GOOS,
		Arch: p.Build.GOARCH,
		Outputs: fns.Map(res.Outputs, func(o *builder.JSBuildOutput) builder.BuildOutput {
			return o
		}),
	}, nil
}

func (i *BuilderImpl) UseNewRuntimeConfig() bool {
	return true
}

func (i *BuilderImpl) NeedsMeta() bool {
	return true
}

func (i *BuilderImpl) ServiceConfigs(ctx context.Context, p builder.ServiceConfigsParams) (*builder.ServiceConfigsResult, error) {
	return &builder.ServiceConfigsResult{
		// These are not currently supported
		Configs:     nil,
		ConfigFiles: nil,
	}, nil
}

type testInput struct {
	RuntimeVersion  string `json:"runtime_version"`
	UseLocalRuntime bool   `json:"use_local_runtime"`
}

func (i *BuilderImpl) RunTests(ctx context.Context, p builder.RunTestsParams) error {
	cmd := exec.CommandContext(ctx, p.Spec.Command, p.Spec.Args...)
	cmd.Env = p.Spec.Environ
	cmd.Dir = p.WorkingDir.ToIO()
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	return cmd.Run()
}

func (i *BuilderImpl) TestSpec(ctx context.Context, p builder.TestSpecParams) (*builder.TestSpecResult, error) {
	data := p.Compile.Parse.Data.(*data)

	input, _ := json.Marshal(testInput{
		RuntimeVersion:  version.Version,
		UseLocalRuntime: p.Compile.Build.UseLocalJSRuntime,
	})

	_, _ = data.stdin.Write([]byte("test\n"))
	if _, err := data.stdin.Write(input); err != nil {
		return nil, fmt.Errorf("unable to write to stdin: %s", err)
	}

	isSuccess, testResp, err := readResp(data.stdout)
	if err != nil {
		return nil, fmt.Errorf("unable to read response: %s", err)
	} else if !isSuccess {
		return nil, errors.New(string(testResp))
	}

	var res struct {
		Cmd option.Option[*builder.CmdSpec] `json:"cmd"`
	}
	if err := json.Unmarshal(testResp, &res); err != nil {
		return nil, fmt.Errorf("unable to decode response: %s", err)
	}

	cmdSpec, ok := res.Cmd.Get()
	if !ok {
		return nil, builder.ErrNoTests
	}

	command := cmdSpec.Command.Expand("")
	args := append(command[1:], p.Args...)

	// Default to the error log level to avoid spamming the logs.
	envs := append(os.Environ(), "ENCORE_LOG_LEVEL=error")

	envs = append(envs, p.Env...)
	envs = append(envs, cmdSpec.Env.Expand("")...)

	return &builder.TestSpecResult{
		Command:     command[0],
		Args:        args,
		Environ:     envs,
		BuilderData: nil,
	}, nil
}

func (i *BuilderImpl) GenUserFacing(ctx context.Context, p builder.GenUserFacingParams) error {
	data := p.Parse.Data.(*data)

	input, _ := json.Marshal(genUserFacingInput{})

	_, _ = data.stdin.Write([]byte("gen-user-facing\n"))
	if _, err := data.stdin.Write(input); err != nil {
		return fmt.Errorf("unable to write to stdin: %s", err)
	}

	isSuccess, genUserFacingRep, err := readResp(data.stdout)
	if err != nil {
		return fmt.Errorf("unable to read response: %s", err)
	} else if !isSuccess {
		return errors.New(string(genUserFacingRep))
	}
	return nil
}

type genUserFacingInput struct {
}

type prepareInput struct {
	JSRuntimeRoot   paths.FS `json:"js_runtime_root"`
	AppRoot         paths.FS `json:"app_root"`
	RuntimeVersion  string   `json:"runtime_version"`
	UseLocalRuntime bool     `json:"use_local_runtime"`
}

func readResp(reader io.Reader) (isSuccess bool, data []byte, err error) {
	var respLen uint32
	if err := binary.Read(reader, binary.LittleEndian, &respLen); err != nil {
		return false, nil, fmt.Errorf("unable to read response length: %s", err)
	}
	resp := make([]byte, respLen)
	if _, err := io.ReadFull(reader, resp); err != nil {
		return false, nil, fmt.Errorf("unable to read response: %s", err)
	}

	isSuccess, data = resp[0] == 0, resp[1:]
	return isSuccess, data, nil
}

// Reports the JS Runtime root directory.
func jsRuntimeRoot(runtimesPath paths.FS) paths.FS {
	return runtimesPath.Join("js")
}

type runningCmd struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

func newRunningCmd(c *exec.Cmd) *runningCmd {
	rc := &runningCmd{
		cmd:  c,
		done: make(chan struct{}),
		err:  nil,
	}
	go func() {
		defer close(rc.done)
		rc.err = c.Wait()
	}()
	return rc
}
