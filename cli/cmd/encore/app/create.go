package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tailscale/hujson"

	"encr.dev/cli/cmd/encore/auth"
	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"encr.dev/internal/conf"
	"encr.dev/internal/env"
)

var createAppTemplate string

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
	createAppCmd.Flags().StringVar(&createAppTemplate, "example", "", "URL to example code to use.")
}

// createApp is the implementation of the "encore app create" command.
func createApp(ctx context.Context, name, template string) (err error) {
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)

	if _, err := conf.CurrentUser(); errors.Is(err, fs.ErrNotExist) {
		cyan.Fprint(os.Stderr, "Log in to create your app [press enter to continue]: ")
		fmt.Scanln()
		if err := auth.DoLogin(); err != nil {
			cmdutil.Fatal(err)
		}
	}

	if name == "" || template == "" {
		name, template = selectTemplate(name, template)
	}

	if err := validateName(name); err != nil {
		return err
	} else if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("directory %s already exists", name)
	}

	// Parse template information, if provided.
	var ex *repoInfo
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
			os.RemoveAll(name)
		}
	}()

	if ex != nil {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = fmt.Sprintf("Downloading template %s ", ex.Name)
		s.Start()
		err := downloadAndExtractTemplate(ctx, name, *ex)
		s.Stop()
		fmt.Println()

		if err != nil {
			return fmt.Errorf("failed to download template %s: %v", ex.Name, err)
		}
		gray := color.New(color.Faint)
		gray.Printf("Downloaded template %s.\n", ex.Name)
	} else {
		// Set up files that we need when we don't have an example
		if err := os.WriteFile(filepath.Join(name, ".gitignore"), []byte("/.encore\n"), 0644); err != nil {
			cmdutil.Fatal(err)
		}
		encoreModData := []byte("module encore.app\n")
		if err := os.WriteFile(filepath.Join(name, "go.mod"), encoreModData, 0644); err != nil {
			cmdutil.Fatal(err)
		}
	}

	// Create the app on the server.
	_, err = conf.CurrentUser()
	loggedIn := err == nil

	var app *platform.App
	if loggedIn {
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

	// Create the encore.app file
	var encoreAppData []byte
	if loggedIn {
		encoreAppData = []byte(`{
	"id": "` + app.Slug + `",
}
`)
	} else {
		encoreAppData = []byte(`{
	// The app is not currently linked to the encore.dev platform.
	// Use "encore app link" to link it.
	"id": "",
}
`)
	}
	if err := os.WriteFile(filepath.Join(name, "encore.app"), encoreAppData, 0644); err != nil {
		return err
	}

	// Update to latest encore.dev release
	{
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = "Running go get encore.dev@latest"
		s.Start()
		if err := gogetEncore(name); err != nil {
			s.FinalMSG = fmt.Sprintf("failed, skipping: %v", err.Error())
		}
		s.Stop()
	}

	// Rewrite any existence of ENCORE_APP_ID to the allocated app id.
	if app != nil {
		if err := rewritePlaceholders(name, app); err != nil {
			red := color.New(color.FgRed)
			red.Printf("Failed rewriting source code placeholders, skipping: %v\n", err)
		}
	}

	if err := initGitRepo(name, app); err != nil {
		return err
	}

	green.Printf("\nSuccessfully created app %s!\n", name)
	cyanf := cyan.SprintfFunc()
	if app != nil {
		fmt.Printf("App ID:  %s\n", cyanf(app.Slug))
		fmt.Printf("Web URL: %s%s", cyanf("https://app.encore.dev/"+app.Slug), cmdutil.Newline)
	}

	fmt.Print("\nUseful commands:\n\n")

	cyan.Printf("    encore run\n")
	fmt.Print("        Run your app locally\n\n")

	cyan.Printf("    encore test ./...\n")
	fmt.Print("        Run tests\n\n")

	if app != nil {
		cyan.Printf("    git push encore\n")
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

func gogetEncore(name string) error {
	// Use the 'go' binary from the Encore GOROOT in case the user
	// does not have Go installed separately from Encore.
	goPath := filepath.Join(env.EncoreGoRoot(), "bin", "go")
	cmd := exec.Command(goPath, "get", "encore.dev@latest")
	cmd.Dir = name
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(string(out))
	}
	return nil
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

type repoInfo struct {
	Owner  string
	Repo   string
	Branch string
	Path   string // subdirectory to copy ("." for whole project)
	Name   string // example name
}

func parseTemplate(ctx context.Context, tmpl string) (*repoInfo, error) {
	switch {
	case strings.HasPrefix(tmpl, "http"):
		// Already an URL; do nothing
	case strings.HasPrefix(tmpl, "github.com"):
		// Assume a URL without the scheme
		tmpl = "https://" + tmpl
	default:
		// Simple template name
		tmpl = "https://github.com/encoredev/examples/tree/main/" + tmpl
	}

	u, err := url.Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %v", err)
	}
	if u.Host != "github.com" {
		return nil, fmt.Errorf("template must be hosted on GitHub, not %s", u.Host)
	}
	// Path must be one of:
	// "/owner/repo"
	// "/owner/repo/tree/<branch>"
	// "/owner/repo/tree/<branch>/path"
	parts := strings.SplitN(u.Path, "/", 6)
	switch {
	case len(parts) == 3: // "/owner/repo"
		owner, repo := parts[1], parts[2]
		// Check the default branch
		var resp struct {
			DefaultBranch string `json:"default_branch"`
		}
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		} else if err := slurpJSON(req, &resp); err != nil {
			return nil, err
		}
		return &repoInfo{
			Owner:  owner,
			Repo:   repo,
			Branch: resp.DefaultBranch,
			Path:   ".",
			Name:   repo,
		}, nil
	case len(parts) >= 5: // "/owner/repo"
		owner, repo, t, branch := parts[1], parts[2], parts[3], parts[4]
		p := "."
		name := repo
		if len(parts) == 6 {
			p = parts[5]
			name = path.Base(p)
		}
		if t != "tree" {
			return nil, fmt.Errorf("unsupported template url: %s", tmpl)
		}
		return &repoInfo{
			Owner:  owner,
			Repo:   repo,
			Branch: branch,
			Path:   p,
			Name:   name,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported template url: %s", tmpl)
	}
}

func downloadAndExtractTemplate(ctx context.Context, dst string, info repoInfo) error {
	url := fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/%s", info.Owner, info.Repo, info.Branch)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("GET %s: got non-200 response: %s", url, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read gzip response: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	prefix := path.Join(info.Repo+"-"+info.Branch, info.Path)
	prefix += "/"
	files := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			if files == 0 {
				return fmt.Errorf("could not find template")
			}
			return nil
		} else if err != nil {
			return fmt.Errorf("reading repo data: %v", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		if p := path.Clean(hdr.Name); strings.HasPrefix(p, prefix) {
			files++
			p = p[len(prefix):]
			filePath := filepath.Join(dst, filepath.FromSlash(p))
			if err := createFile(tr, filePath); err != nil {
				return fmt.Errorf("create %s: %v", p, err)
			}
		}
	}
}

func createFile(src io.Reader, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, src)
	if err2 := f.Close(); err == nil {
		err = err2
	}
	return err
}

func slurpJSON(req *http.Request, respData interface{}) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("got non-200 response: %s: %s", resp.Status, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(respData); err != nil {
		return fmt.Errorf("could not decode response: %v", err)
	}
	return nil
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
		if err != nil && err != exec.ErrNotFound {
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
	if out, err := cmd.CombinedOutput(); err != nil && err != exec.ErrNotFound {
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
		return os.WriteFile(path, data, info.Type().Perm())
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
