package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/cockroachdb/errors"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tailscale/hujson"

	"encr.dev/cli/cmd/encore/auth"
	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"encr.dev/internal/conf"
	"encr.dev/internal/env"
	"encr.dev/internal/version"
	"encr.dev/pkg/github"
	"encr.dev/pkg/xos"
)

var (
	createAppTemplate   string
	createAppOnPlatform bool
)

var createAppCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new Encore app",
	Args:  cobra.MaximumNArgs(1),

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if err := createApp(context.Background(), name, createAppTemplate); err != nil {
			cmdutil.Fatal(err)
		}
	},
}

func init() {
	appCmd.AddCommand(createAppCmd)
	createAppCmd.Flags().BoolVar(&createAppOnPlatform, "platform", true, "whether to create the app with the Encore Platform")
	createAppCmd.Flags().StringVar(&createAppTemplate, "example", "", "URL to example code to use.")
}

// createApp is the implementation of the "encore app create" command.
func createApp(ctx context.Context, name, template string) (err error) {
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)

	if _, err := conf.CurrentUser(); errors.Is(err, fs.ErrNotExist) && createAppOnPlatform {
		_, _ = cyan.Fprint(os.Stderr, "Log in to create your app [press enter to continue]: ")
		_, _ = fmt.Scanln()
		if err := auth.DoLogin(auth.AutoFlow); err != nil {
			cmdutil.Fatal(err)
		}
	}

	if name == "" || template == "" {
		name, template = selectTemplate(name, template)
	}
	// Treat the special name "empty" as the empty app template
	// (the rest of the code assumes that's the empty string).
	if template == "empty" {
		template = ""
	}

	if err := validateName(name); err != nil {
		return err
	} else if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("directory %s already exists", name)
	}

	// Parse template information, if provided.
	var ex *github.Tree
	if template != "" {
		var err error
		ex, err = parseTemplate(ctx, template)
		if err != nil {
			return err
		}
	}

	if err := os.Mkdir(name, 0755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			// Clean up the directory we just created in case of an error.
			_ = os.RemoveAll(name)
		}
	}()

	if ex != nil {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = fmt.Sprintf("Downloading template %s ", ex.Name())
		s.Start()
		err := github.ExtractTree(ctx, ex, name)
		s.Stop()
		fmt.Println()

		if err != nil {
			return fmt.Errorf("failed to download template %s: %v", ex.Name(), err)
		}
		gray := color.New(color.Faint)
		_, _ = gray.Printf("Downloaded template %s.\n", ex.Name())
	} else {
		// Set up files that we need when we don't have an example
		if err := xos.WriteFile(filepath.Join(name, ".gitignore"), []byte("/.encore\n"), 0644); err != nil {
			cmdutil.Fatal(err)
		}
		encoreModData := []byte("module encore.app\n")
		if err := xos.WriteFile(filepath.Join(name, "go.mod"), encoreModData, 0644); err != nil {
			cmdutil.Fatal(err)
		}
	}

	// Create the app on the server.
	_, err = conf.CurrentUser()
	loggedIn := err == nil

	var app *platform.App
	if loggedIn && createAppOnPlatform {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = "Creating app on encore.dev "
		s.Start()

		exCfg, ok := parseExampleConfig(name)
		app, err = createAppOnServer(name, exCfg)
		s.Stop()
		if err != nil {
			return fmt.Errorf("creating app on encore.dev: %v", err)
		}

		// Remove the example.json file if the app was successfully created.
		if ok {
			_ = os.Remove(exampleJSONPath(name))
		}
	}

	encoreAppPath := filepath.Join(name, "encore.app")
	appData, err := os.ReadFile(encoreAppPath)
	if err != nil {
		appData, err = []byte("{}"), nil
	}

	if app != nil {
		appData, err = setEncoreAppID(appData, app.Slug, []string{})
	} else {
		appData, err = setEncoreAppID(appData, "", []string{
			"The app is not currently linked to the encore.dev platform.",
			`Use "encore app link" to link it.`,
		})
	}
	if err != nil {
		return errors.Wrap(err, "write encore.app file")
	}
	if err := xos.WriteFile(encoreAppPath, appData, 0644); err != nil {
		return errors.Wrap(err, "write encore.app file")
	}

	// Update to latest encore.dev release
	if _, err := os.Stat(filepath.Join(name, "go.mod")); err == nil {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = "Running go get encore.dev@latest"
		s.Start()
		if err := gogetEncore(name); err != nil {
			s.FinalMSG = fmt.Sprintf("failed, skipping: %v", err.Error())
		}
		s.Stop()
	} else if _, err := os.Stat(filepath.Join(name, "package.json")); err == nil {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = "Running npm install encore.dev@latest"
		s.Start()
		if err := npmInstallEncore(name); err != nil {
			s.FinalMSG = fmt.Sprintf("failed, skipping: %v", err.Error())
		}
		s.Stop()
	}

	// Rewrite any existence of ENCORE_APP_ID to the allocated app id.
	if app != nil {
		if err := rewritePlaceholders(name, app); err != nil {
			red := color.New(color.FgRed)
			_, _ = red.Printf("Failed rewriting source code placeholders, skipping: %v\n", err)
		}
	}

	if err := initGitRepo(name, app); err != nil {
		return err
	}

	// Try to generate wrappers. Don't error out if it fails for some reason,
	// it's a nice-to-have to avoid IDEs thinking there are compile errors before 'encore run' runs.
	_ = generateWrappers(name)

	_, _ = green.Printf("\nSuccessfully created app %s!\n", name)
	cyanf := cyan.SprintfFunc()
	if app != nil {
		fmt.Printf("App ID:  %s\n", cyanf(app.Slug))
		fmt.Printf("Web URL: %s%s", cyanf("https://app.encore.dev/"+app.Slug), cmdutil.Newline)
	}

	fmt.Print("\nUseful commands:\n\n")

	_, _ = cyan.Printf("    encore run\n")
	fmt.Print("        Run your app locally\n\n")

	_, _ = cyan.Printf("    encore test ./...\n")
	fmt.Print("        Run tests\n\n")

	if app != nil {
		_, _ = cyan.Printf("    git push encore\n")
		fmt.Print("        Deploys your app\n\n")
	}

	greenBoldF := green.Add(color.Bold).SprintfFunc()
	fmt.Printf("Get started now: %s\n", greenBoldF("cd %s && encore run", name))

	return nil
}

func validateName(name string) error {
	ln := len(name)
	if ln == 0 {
		return fmt.Errorf("name must not be empty")
	} else if ln > 50 {
		return fmt.Errorf("name too long (max 50 chars)")
	}

	for i, s := range name {
		// Outside of [a-z], [0-9] and != '-'?
		if !((s >= 'a' && s <= 'z') || (s >= '0' && s <= '9') || s == '-') {
			return fmt.Errorf("name must only contain lowercase letters, digits, or dashes")
		} else if s == '-' {
			if i == 0 {
				return fmt.Errorf("name cannot start with a dash")
			} else if (i + 1) == ln {
				return fmt.Errorf("name cannot end with a dash")
			} else if name[i-1] == '-' {
				return fmt.Errorf("name cannot contain repeated dashes")
			}
		}
	}
	return nil
}

func gogetEncore(dir string) error {
	var goBinPath string

	// Prefer the 'go' binary from the Encore GOROOT if available.
	if goroot, ok := env.OptEncoreGoRoot().Get(); ok {
		goBinPath = filepath.Join(goroot, "bin", "go")
	} else {
		// Otherwise fall back to just "go", so that exec.Command
		// does a path lookup.
		goBinPath = "go"
	}

	// Use the 'go' binary from the Encore GOROOT in case the user
	// does not have Go installed separately from Encore.
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(goBinPath, "get", "encore.dev@latest")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(string(out))
	}
	return nil
}

func npmInstallEncore(dir string) error {
	verToInstall := version.Version
	if version.Channel == version.DevBuild {
		verToInstall = "latest"
	}

	// First install the 'encore.dev' package.
	cmd := exec.Command("npm", "install", fmt.Sprintf("encore.dev@%s", verToInstall))
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("'npm install encore.dev@%s' failed: %v: %s", verToInstall, err, out)
	}

	// Then run 'npm install'.
	cmd = exec.Command("npm install")
	cmd.Dir = dir
	if out2, err2 := cmd.CombinedOutput(); err2 != nil && err == nil {
		err = fmt.Errorf("'npm install' failed: %v: %s", err2, out2)
	}

	return err
}

func createAppOnServer(name string, cfg exampleConfig) (*platform.App, error) {
	if _, err := conf.CurrentUser(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	params := &platform.CreateAppParams{
		Name:           name,
		InitialSecrets: cfg.InitialSecrets,
	}
	return platform.CreateApp(ctx, params)
}

func parseTemplate(ctx context.Context, tmpl string) (*github.Tree, error) {
	// If the template does not contain a colon or a dot, it's definitely
	// not a github.com URL. Assume it's a simple template name.
	if !strings.Contains(tmpl, ":") && !strings.Contains(tmpl, ".") {
		tmpl = "https://github.com/encoredev/examples/tree/main/" + tmpl
	}
	return github.ParseTree(ctx, tmpl)
}

// initGitRepo initializes the git repo.
// If app is not nil, it configures the repo to push to the given app.
// If git does not exist, it reports an error matching exec.ErrNotFound.
func initGitRepo(path string, app *platform.App) (err error) {
	defer func() {
		if e := recover(); e != nil {
			if ee, ok := e.(error); ok {
				err = ee
			} else {
				panic(e)
			}
		}
	}()

	git := func(args ...string) []byte {
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		out, err := cmd.CombinedOutput()
		if err != nil && !errors.Is(err, exec.ErrNotFound) {
			panic(fmt.Errorf("git %s: %s (%w)", strings.Join(args, " "), out, err))
		}
		return out
	}

	// Initialize git repo
	git("init")
	if app != nil && app.MainBranch != nil {
		git("checkout", "-b", *app.MainBranch)
	}
	git("config", "--local", "push.default", "current")
	git("add", "-A")

	cmd := exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = path
	// Configure the committer if the user hasn't done it themselves yet.
	if ok, _ := gitUserConfigured(); !ok {
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Encore",
			"GIT_AUTHOR_EMAIL=git-bot@encore.dev",
			"GIT_COMMITTER_NAME=Encore",
			"GIT_COMMITTER_EMAIL=git-bot@encore.dev",
		)
	}
	if out, err := cmd.CombinedOutput(); err != nil && !errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("create initial commit repository: %s (%v)", out, err)
	}

	if app != nil {
		git("remote", "add", defaultGitRemoteName, defaultGitRemoteURL+app.Slug)
	}

	return nil
}

func addEncoreRemote(root, appID string) {
	// Determine if there are any remotes
	cmd := exec.Command("git", "remote")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		cmd = exec.Command("git", "remote", "add", defaultGitRemoteName, defaultGitRemoteURL+appID)
		cmd.Dir = root
		if err := cmd.Run(); err == nil {
			fmt.Println("Configured git remote 'encore' to push/pull with Encore.")
		}
	}
}

// gitUserConfigured reports whether the user has configured
// user.name and user.email in git.
func gitUserConfigured() (bool, error) {
	for _, s := range []string{"user.name", "user.email"} {
		out, err := exec.Command("git", "config", s).CombinedOutput()
		if err != nil {
			return false, err
		} else if len(bytes.TrimSpace(out)) == 0 {
			return false, nil
		}
	}
	return true, nil
}

// rewritePlaceholders recursively rewrites all files within basePath
// to replace placeholders with the actual values for this particular app.
func rewritePlaceholders(basePath string, app *platform.App) error {
	var first error
	err := filepath.WalkDir(basePath, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if err := rewritePlaceholder(path, info, app); err != nil {
			if first == nil {
				first = err
			}
		}
		return nil
	})
	if err == nil {
		err = first
	}
	return err
}

// rewritePlaceholder rewrites a file to replace placeholders with the
// actual values for this particular app. If the file contains none of
// the placeholders, this is a no-op.
func rewritePlaceholder(path string, info fs.DirEntry, app *platform.App) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	placeholders := []string{
		"{{ENCORE_APP_ID}}", app.Slug,
	}

	var replaced bool
	for i := 0; i < len(placeholders); i += 2 {
		placeholder := []byte(placeholders[i])
		target := []byte(placeholders[i+1])
		if bytes.Contains(data, placeholder) {
			data = bytes.ReplaceAll(data, placeholder, target)
			replaced = true
		}
	}

	if replaced {
		return xos.WriteFile(path, data, info.Type().Perm())
	}
	return nil
}

// exampleConfig is the optional configuration file for example apps.
type exampleConfig struct {
	InitialSecrets map[string]string `json:"initial_secrets"`
}

func parseExampleConfig(repoPath string) (cfg exampleConfig, exists bool) {
	if data, err := os.ReadFile(exampleJSONPath(repoPath)); err == nil {
		if data, err = hujson.Standardize(data); err == nil {
			if err := json.Unmarshal(data, &cfg); err == nil {
				return cfg, true
			}
		}
	}
	return exampleConfig{}, false
}

func exampleJSONPath(repoPath string) string {
	return filepath.Join(repoPath, "example-initial-setup.json")
}

// setEncoreAppID rewrites the encore.app file to replace the app id, preserving comments.
// It optionally adds comment lines before the "id" field if commentLines is not nil.
func setEncoreAppID(data []byte, id string, commentLines []string) ([]byte, error) {
	if len(data) == 0 {
		data = []byte("{}")
	}

	root, err := hujson.Parse(data)
	if err != nil {
		return data, errors.Wrap(err, "parse encore.app")
	}
	obj, ok := root.Value.(*hujson.Object)
	if !ok {
		return data, errors.New("invalid encore.app format: not a json object")
	}

	var buf bytes.Buffer
	for i, ln := range commentLines {
		if i == 0 {
			fmt.Fprintf(&buf, "\n")
		}
		fmt.Fprintf(&buf, "\t// %s\n", strings.TrimSpace(ln))
	}
	extra := hujson.Extra(buf.Bytes())
	jsonValue, _ := json.Marshal(id)
	value := hujson.Value{
		Value: hujson.Literal(jsonValue),
	}

	found := false
	for i := range obj.Members {
		m := &obj.Members[i]
		if lit, ok := m.Name.Value.(hujson.Literal); ok && lit.String() == "id" {
			if commentLines != nil {
				m.Name.BeforeExtra = extra
			}
			m.Value = value
			found = true
			break
		}
	}

	if !found {
		obj.Members = append([]hujson.ObjectMember{{
			Name: hujson.Value{
				BeforeExtra: extra,
				Value:       hujson.Literal(`"id"`),
			},
			Value: value,
		}}, obj.Members...)
	}

	root.Format()
	return root.Pack(), nil
}

// generateWrappers runs 'encore gen wrappers' in the given directory.
func generateWrappers(dir string) error {
	// Use this executable if we can.
	exe, err := os.Executable()
	if err != nil {
		exe = "encore"
	}
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(exe, "gen", "wrappers")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("encore gen wrappers failed: %v: %s", err, out)
	}
	return nil
}
