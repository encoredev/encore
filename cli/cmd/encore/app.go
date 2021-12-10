package main

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
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"encr.dev/cli/internal/conf"
	"encr.dev/cli/internal/platform"
	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tailscale/hujson"
)

func init() {
	appCmd := &cobra.Command{
		Use:   "app",
		Short: "Commands to create and link Encore apps",
	}
	rootCmd.AddCommand(appCmd)

	var createAppTemplate string

	createAppCmd := &cobra.Command{
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
				fatal(err)
			}
		},
	}
	appCmd.AddCommand(createAppCmd)
	createAppCmd.Flags().StringVar(&createAppTemplate, "example", "", "URL to example code to use.")

	var forceLink bool
	linkAppCmd := &cobra.Command{
		Use:   "link [app-id]",
		Short: "Link an Encore app with the server",
		Args:  cobra.MaximumNArgs(1),

		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			var appID string
			if len(args) > 0 {
				appID = args[0]
			}
			linkApp(appID, forceLink)
		},
	}
	appCmd.AddCommand(linkAppCmd)
	linkAppCmd.Flags().BoolVarP(&forceLink, "force", "f", false, "Force link even if the app is already linked.")

	cloneAppCmd := &cobra.Command{
		Use:   "clone [app-id] [directory]",
		Short: "Clone an Encore app to your computer",
		Args:  cobra.MinimumNArgs(1),

		DisableFlagsInUseLine: true,
		Run: func(c *cobra.Command, args []string) {
			cmdArgs := append([]string{"clone", "encore://" + args[0]}, args[1:]...)
			cmd := exec.Command("git", cmdArgs...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				os.Exit(1)
			}
		},
	}

	appCmd.AddCommand(cloneAppCmd)
}

// createApp is the implementation of the "encore app create" command.
func createApp(ctx context.Context, name, template string) (err error) {
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)

	if _, err := conf.CurrentUser(); errors.Is(err, fs.ErrNotExist) {
		cyan.Fprint(os.Stderr, "Log in to create your app [press enter to continue]: ")
		fmt.Scanln()
		if err := doLogin(); err != nil {
			fatal(err)
		}
	}

	if name == "" {
		err := survey.AskOne(&survey.Input{
			Message: "App Name (lowercase letters, digits, and dashes)",
		}, &name, survey.WithValidator(func(in interface{}) error { return validateName(in.(string)) }))
		if err != nil {
			if err.Error() == "interrupt" {
				os.Exit(2)
			}
			return err
		}
	}

	if err := validateName(name); err != nil {
		return err
	} else if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("directory %s already exists", name)
	}

	if template == "" {
		var idx int

		prompt := &survey.Select{
			Message: "Select app template:",
			Options: []string{
				"Hello World (Encore introduction)",
				"Empty app",
			},
		}
		survey.AskOne(prompt, &idx)
		switch idx {
		case 0:
			template = "hello-world"
		case 1:
			template = ""
		}
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
		if err := ioutil.WriteFile(filepath.Join(name, ".gitignore"), []byte("/.encore\n"), 0644); err != nil {
			fatal(err)
		}
		encoreModData := []byte("module encore.app\n")
		if err := ioutil.WriteFile(filepath.Join(name, "go.mod"), encoreModData, 0644); err != nil {
			fatal(err)
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
		app, err = createAppOnServer(name)
		s.Stop()
		if err != nil {
			return fmt.Errorf("creating app on encore.dev: %v", err)
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
	if err := ioutil.WriteFile(filepath.Join(name, "encore.app"), encoreAppData, 0644); err != nil {
		return err
	}

	if err := initGitRepo(name, app); err != nil {
		return err
	}

	green.Printf("\nSuccessfully created app %s!\n", name)
	cyanf := cyan.SprintfFunc()
	if app != nil {
		fmt.Printf("App ID:  %s\n", cyanf(app.Slug))
		fmt.Printf("Web URL: %s%s", cyanf("https://app.encore.dev/"+app.Slug), newline)
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

type appConf struct {
	Slug          string  `json:"slug"`
	DefaultBranch *string `json:"main_branch"`
}

func createAppOnServer(name string) (*platform.App, error) {
	if _, err := conf.CurrentUser(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	params := &platform.CreateAppParams{
		Name: name,
	}
	return platform.CreateApp(ctx, params)
}

func validateAppSlug(slug string) (ok bool, err error) {
	if _, err := conf.CurrentUser(); errors.Is(err, fs.ErrNotExist) {
		fatal("not logged in. Run 'encore auth login' first.")
	} else if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := platform.GetApp(ctx, slug); err != nil {
		var e platform.Error
		if errors.As(err, &e) && e.HTTPCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
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
	resp, err := conf.AuthClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
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
		git("remote", "add", "encore", "encore://"+app.Slug)
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
		cmd = exec.Command("git", "remote", "add", "encore", "encore://"+appID)
		cmd.Dir = root
		if err := cmd.Run(); err == nil {
			fmt.Println("Configured git remote 'encore' to push/pull with Encore.")
		}
	}
}

func linkApp(appID string, force bool) {
	root, _ := determineAppRoot()
	filePath := filepath.Join(root, "encore.app")

	// Parse the app data using a map so we preserve all
	// the keys present when writing it back below.
	var appData map[string]interface{}
	if data, err := ioutil.ReadFile(filePath); err != nil {
		fatal(err)
		os.Exit(1)
	} else if err := hujson.Unmarshal(data, &appData); err != nil {
		fatal("could not parse encore.app: ", err)
		os.Exit(1)
	} else if appData["id"] != nil && appData["id"] != "" {
		fatal("the app is already linked.\n\nNote: to link to a different app, specify the --force flag.")
	}

	if appID == "" {
		fmt.Println("Make sure the app is created on app.encore.dev, and then enter its ID to link it.")
		fmt.Print("App ID: ")
		if _, err := fmt.Scanln(&appID); err != nil {
			fatal(err)
		} else if appID == "" {
			fatal("no app id given.")
		}
	}

	if linked, err := validateAppSlug(appID); err != nil {
		fatal(err)
	} else if !linked {
		fmt.Fprintln(os.Stderr, "Error: that app does not exist, or you don't have access to it.")
		os.Exit(1)
	}

	appData["id"] = appID
	data, _ := hujson.MarshalIndent(appData, "", "    ")
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		fatal(err)
		os.Exit(1)
	}

	addEncoreRemote(root, appID)
	fmt.Println("Successfully linked app!")
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
