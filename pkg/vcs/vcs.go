// Simplified version of the version from Go Get:
// https://github.com/golang/go/blob/f87e28d1b9ab33491b32255f333f1f1d83eeb6fc/src/cmd/go/internal/vcs/vcs.go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vcs

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	urlpkg "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A Cmd describes how to use a version control system
// like Mercurial, Git, or Subversion.
type Cmd struct {
	Name      string
	Cmd       string   // name of binary to invoke command
	RootNames []string // filename indicating the root of a checkout directory

	CreateCmd   []string // commands to download a fresh copy of a repository
	DownloadCmd []string // commands to download updates into an existing repository

	TagCmd         []tagCmd // commands to list tags
	TagLookupCmd   []tagCmd // commands to lookup tags before running tagSyncCmd
	TagSyncCmd     []string // commands to sync to specific tag
	TagSyncDefault []string // commands to sync to default tag

	Scheme  []string
	PingCmd string

	RemoteRepo  func(v *Cmd, rootDir string) (remoteRepo string, err error)
	ResolveRepo func(v *Cmd, rootDir, remoteRepo string) (realRepo string, err error)
	Status      func(v *Cmd, rootDir string) (Status, error)
}

// Status is the current state of a local repository.
type Status struct {
	Revision    string    // Optional.
	CommitTime  time.Time // Optional.
	Uncommitted bool      // Required.
}

var defaultSecureScheme = map[string]bool{
	"https":   true,
	"git+ssh": true,
	"bzr+ssh": true,
	"svn+ssh": true,
	"ssh":     true,
}

func (v *Cmd) IsSecure(repo string) bool {
	u, err := urlpkg.Parse(repo)
	if err != nil {
		// If repo is not a URL, it's not secure.
		return false
	}
	return v.isSecureScheme(u.Scheme)
}

func (v *Cmd) isSecureScheme(scheme string) bool {
	switch v.Cmd {
	case "git":
		// GIT_ALLOW_PROTOCOL is an environment variable defined by Git. It is a
		// colon-separated list of schemes that are allowed to be used with git
		// fetch/clone. Any scheme not mentioned will be considered insecure.
		if allow := os.Getenv("GIT_ALLOW_PROTOCOL"); allow != "" {
			for _, s := range strings.Split(allow, ":") {
				if s == scheme {
					return true
				}
			}
			return false
		}
	}
	return defaultSecureScheme[scheme]
}

// A tagCmd describes a command to list available tags
// that can be passed to tagSyncCmd.
type tagCmd struct {
	cmd     string // command to list tags
	pattern string // regexp to extract tags from list
}

// vcsList lists the known version control systems
var vcsList = []*Cmd{
	vcsHg,
	vcsGit,
	vcsSvn,
	vcsBzr,
	vcsFossil,
}

// vcsHg describes how to use Mercurial.
var vcsHg = &Cmd{
	Name:      "Mercurial",
	Cmd:       "hg",
	RootNames: []string{".hg"},

	CreateCmd:   []string{"clone -U -- {repo} {dir}"},
	DownloadCmd: []string{"pull"},

	// We allow both tag and branch names as 'tags'
	// for selecting a version. This lets people have
	// a go.release.r60 branch and a go1 branch
	// and make changes in both, without constantly
	// editing .hgtags.
	TagCmd: []tagCmd{
		{"tags", `^(\S+)`},
		{"branches", `^(\S+)`},
	},
	TagSyncCmd:     []string{"update -r {tag}"},
	TagSyncDefault: []string{"update default"},

	Scheme:     []string{"https", "http", "ssh"},
	PingCmd:    "identify -- {scheme}://{repo}",
	RemoteRepo: hgRemoteRepo,
	Status:     hgStatus,
}

func hgRemoteRepo(vcsHg *Cmd, rootDir string) (remoteRepo string, err error) {
	out, err := vcsHg.runOutput(rootDir, "paths default")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func hgStatus(vcsHg *Cmd, rootDir string) (Status, error) {
	// Output changeset ID and seconds since epoch.
	out, err := vcsHg.runOutputVerboseOnly(rootDir, `log -l1 -T {node}:{date|hgdate}`)
	if err != nil {
		return Status{}, err
	}

	// Successful execution without output indicates an empty repo (no commits).
	var rev string
	var commitTime time.Time
	if len(out) > 0 {
		// Strip trailing timezone offset.
		if i := bytes.IndexByte(out, ' '); i > 0 {
			out = out[:i]
		}
		rev, commitTime, err = parseRevTime(out)
		if err != nil {
			return Status{}, err
		}
	}

	// Also look for untracked files.
	out, err = vcsHg.runOutputVerboseOnly(rootDir, "status")
	if err != nil {
		return Status{}, err
	}
	uncommitted := len(out) > 0

	return Status{
		Revision:    rev,
		CommitTime:  commitTime,
		Uncommitted: uncommitted,
	}, nil
}

// parseRevTime parses commit details in "revision:seconds" format.
func parseRevTime(out []byte) (string, time.Time, error) {
	buf := string(bytes.TrimSpace(out))

	i := strings.IndexByte(buf, ':')
	if i < 1 {
		return "", time.Time{}, errors.New("unrecognized VCS tool output")
	}
	rev := buf[:i]

	secs, err := strconv.ParseInt(string(buf[i+1:]), 10, 64)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("unrecognized VCS tool output: %v", err)
	}

	return rev, time.Unix(secs, 0), nil
}

// vcsGit describes how to use Git.
var vcsGit = &Cmd{
	Name:      "Git",
	Cmd:       "git",
	RootNames: []string{".git"},

	CreateCmd:   []string{"clone -- {repo} {dir}", "-go-internal-cd {dir} submodule update --init --recursive"},
	DownloadCmd: []string{"pull --ff-only", "submodule update --init --recursive"},

	TagCmd: []tagCmd{
		// tags/xxx matches a git tag named xxx
		// origin/xxx matches a git branch named xxx on the default remote repository
		{"show-ref", `(?:tags|origin)/(\S+)$`},
	},
	TagLookupCmd: []tagCmd{
		{"show-ref tags/{tag} origin/{tag}", `((?:tags|origin)/\S+)$`},
	},
	TagSyncCmd: []string{"checkout {tag}", "submodule update --init --recursive"},
	// both createCmd and downloadCmd update the working dir.
	// No need to do more here. We used to 'checkout master'
	// but that doesn't work if the default branch is not named master.
	// DO NOT add 'checkout master' here.
	// See golang.org/issue/9032.
	TagSyncDefault: []string{"submodule update --init --recursive"},

	Scheme: []string{"git", "https", "http", "git+ssh", "ssh"},

	// Leave out the '--' separator in the ls-remote command: git 2.7.4 does not
	// support such a separator for that command, and this use should be safe
	// without it because the {scheme} value comes from the predefined list above.
	// See golang.org/issue/33836.
	PingCmd: "ls-remote {scheme}://{repo}",

	RemoteRepo: gitRemoteRepo,
	Status:     gitStatus,
}

// scpSyntaxRe matches the SCP-like addresses used by Git to access
// repositories by SSH.
var scpSyntaxRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)@([a-zA-Z0-9._-]+):(.*)$`)

func gitRemoteRepo(vcsGit *Cmd, rootDir string) (remoteRepo string, err error) {
	cmd := "config remote.origin.url"
	errParse := errors.New("unable to parse output of git " + cmd)
	errRemoteOriginNotFound := errors.New("remote origin not found")
	outb, err := vcsGit.run1(rootDir, cmd, nil, false)
	if err != nil {
		// if it doesn't output any message, it means the config argument is correct,
		// but the config value itself doesn't exist
		if outb != nil && len(outb) == 0 {
			return "", errRemoteOriginNotFound
		}
		return "", err
	}
	out := strings.TrimSpace(string(outb))

	var repoURL *urlpkg.URL
	if m := scpSyntaxRe.FindStringSubmatch(out); m != nil {
		// Match SCP-like syntax and convert it to a URL.
		// Eg, "git@github.com:user/repo" becomes
		// "ssh://git@github.com/user/repo".
		repoURL = &urlpkg.URL{
			Scheme: "ssh",
			User:   urlpkg.User(m[1]),
			Host:   m[2],
			Path:   m[3],
		}
	} else {
		repoURL, err = urlpkg.Parse(out)
		if err != nil {
			return "", err
		}
	}

	// Iterate over insecure schemes too, because this function simply
	// reports the state of the repo. If we can't see insecure schemes then
	// we can't report the actual repo URL.
	for _, s := range vcsGit.Scheme {
		if repoURL.Scheme == s {
			return repoURL.String(), nil
		}
	}
	return "", errParse
}

func gitStatus(vcsGit *Cmd, rootDir string) (Status, error) {
	out, err := vcsGit.runOutputVerboseOnly(rootDir, "status --porcelain")
	if err != nil {
		return Status{}, err
	}
	uncommitted := len(out) > 0

	// "git status" works for empty repositories, but "git show" does not.
	// Assume there are no commits in the repo when "git show" fails with
	// uncommitted files and skip tagging revision / committime.
	var rev string
	var commitTime time.Time
	out, err = vcsGit.runOutputVerboseOnly(rootDir, "-c log.showsignature=false show -s --format=%H:%ct")
	if err != nil && !uncommitted {
		return Status{}, err
	} else if err == nil {
		rev, commitTime, err = parseRevTime(out)
		if err != nil {
			return Status{}, err
		}
	}

	return Status{
		Revision:    rev,
		CommitTime:  commitTime,
		Uncommitted: uncommitted,
	}, nil
}

// vcsBzr describes how to use Bazaar.
var vcsBzr = &Cmd{
	Name:      "Bazaar",
	Cmd:       "bzr",
	RootNames: []string{".bzr"},

	CreateCmd: []string{"branch -- {repo} {dir}"},

	// Without --overwrite bzr will not pull tags that changed.
	// Replace by --overwrite-tags after http://pad.lv/681792 goes in.
	DownloadCmd: []string{"pull --overwrite"},

	TagCmd:         []tagCmd{{"tags", `^(\S+)`}},
	TagSyncCmd:     []string{"update -r {tag}"},
	TagSyncDefault: []string{"update -r revno:-1"},

	Scheme:      []string{"https", "http", "bzr", "bzr+ssh"},
	PingCmd:     "info -- {scheme}://{repo}",
	RemoteRepo:  bzrRemoteRepo,
	ResolveRepo: bzrResolveRepo,
	Status:      bzrStatus,
}

func bzrRemoteRepo(vcsBzr *Cmd, rootDir string) (remoteRepo string, err error) {
	outb, err := vcsBzr.runOutput(rootDir, "config parent_location")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(outb)), nil
}

func bzrResolveRepo(vcsBzr *Cmd, rootDir, remoteRepo string) (realRepo string, err error) {
	outb, err := vcsBzr.runOutput(rootDir, "info "+remoteRepo)
	if err != nil {
		return "", err
	}
	out := string(outb)

	// Expect:
	// ...
	//   (branch root|repository branch): <URL>
	// ...

	found := false
	for _, prefix := range []string{"\n  branch root: ", "\n  repository branch: "} {
		i := strings.Index(out, prefix)
		if i >= 0 {
			out = out[i+len(prefix):]
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("unable to parse output of bzr info")
	}

	i := strings.Index(out, "\n")
	if i < 0 {
		return "", fmt.Errorf("unable to parse output of bzr info")
	}
	out = out[:i]
	return strings.TrimSpace(out), nil
}

func bzrStatus(vcsBzr *Cmd, rootDir string) (Status, error) {
	outb, err := vcsBzr.runOutputVerboseOnly(rootDir, "version-info")
	if err != nil {
		return Status{}, err
	}
	out := string(outb)

	// Expect (non-empty repositories only):
	//
	// revision-id: gopher@gopher.net-20211021072330-qshok76wfypw9lpm
	// date: 2021-09-21 12:00:00 +1000
	// ...
	var rev string
	var commitTime time.Time

	for _, line := range strings.Split(out, "\n") {
		i := strings.IndexByte(line, ':')
		if i < 0 {
			continue
		}
		key := line[:i]
		value := strings.TrimSpace(line[i+1:])

		switch key {
		case "revision-id":
			rev = value
		case "date":
			var err error
			commitTime, err = time.Parse("2006-01-02 15:04:05 -0700", value)
			if err != nil {
				return Status{}, errors.New("unable to parse output of bzr version-info")
			}
		}
	}

	outb, err = vcsBzr.runOutputVerboseOnly(rootDir, "status")
	if err != nil {
		return Status{}, err
	}

	// Skip warning when working directory is set to an older revision.
	if bytes.HasPrefix(outb, []byte("working tree is out of date")) {
		i := bytes.IndexByte(outb, '\n')
		if i < 0 {
			i = len(outb)
		}
		outb = outb[:i]
	}
	uncommitted := len(outb) > 0

	return Status{
		Revision:    rev,
		CommitTime:  commitTime,
		Uncommitted: uncommitted,
	}, nil
}

// vcsSvn describes how to use Subversion.
var vcsSvn = &Cmd{
	Name:      "Subversion",
	Cmd:       "svn",
	RootNames: []string{".svn"},

	CreateCmd:   []string{"checkout -- {repo} {dir}"},
	DownloadCmd: []string{"update"},

	// There is no tag command in subversion.
	// The branch information is all in the path names.

	Scheme:     []string{"https", "http", "svn", "svn+ssh"},
	PingCmd:    "info -- {scheme}://{repo}",
	RemoteRepo: svnRemoteRepo,
}

func svnRemoteRepo(vcsSvn *Cmd, rootDir string) (remoteRepo string, err error) {
	outb, err := vcsSvn.runOutput(rootDir, "info")
	if err != nil {
		return "", err
	}
	out := string(outb)

	// Expect:
	//
	//	 ...
	// 	URL: <URL>
	// 	...
	//
	// Note that we're not using the Repository Root line,
	// because svn allows checking out subtrees.
	// The URL will be the URL of the subtree (what we used with 'svn co')
	// while the Repository Root may be a much higher parent.
	i := strings.Index(out, "\nURL: ")
	if i < 0 {
		return "", fmt.Errorf("unable to parse output of svn info")
	}
	out = out[i+len("\nURL: "):]
	i = strings.Index(out, "\n")
	if i < 0 {
		return "", fmt.Errorf("unable to parse output of svn info")
	}
	out = out[:i]
	return strings.TrimSpace(out), nil
}

// fossilRepoName is the name go get associates with a fossil repository. In the
// real world the file can be named anything.
const fossilRepoName = ".fossil"

// vcsFossil describes how to use Fossil (fossil-scm.org)
var vcsFossil = &Cmd{
	Name:      "Fossil",
	Cmd:       "fossil",
	RootNames: []string{".fslckout", "_FOSSIL_"},

	CreateCmd:   []string{"-go-internal-mkdir {dir} clone -- {repo} " + filepath.Join("{dir}", fossilRepoName), "-go-internal-cd {dir} open .fossil"},
	DownloadCmd: []string{"up"},

	TagCmd:         []tagCmd{{"tag ls", `(.*)`}},
	TagSyncCmd:     []string{"up tag:{tag}"},
	TagSyncDefault: []string{"up trunk"},

	Scheme:     []string{"https", "http"},
	RemoteRepo: fossilRemoteRepo,
	Status:     fossilStatus,
}

func fossilRemoteRepo(vcsFossil *Cmd, rootDir string) (remoteRepo string, err error) {
	out, err := vcsFossil.runOutput(rootDir, "remote-url")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

var errFossilInfo = errors.New("unable to parse output of fossil info")

func fossilStatus(vcsFossil *Cmd, rootDir string) (Status, error) {
	outb, err := vcsFossil.runOutputVerboseOnly(rootDir, "info")
	if err != nil {
		return Status{}, err
	}
	out := string(outb)

	// Expect:
	// ...
	// checkout:     91ed71f22c77be0c3e250920f47bfd4e1f9024d2 2021-09-21 12:00:00 UTC
	// ...

	// Extract revision and commit time.
	// Ensure line ends with UTC (known timezone offset).
	const prefix = "\ncheckout:"
	const suffix = " UTC"
	i := strings.Index(out, prefix)
	if i < 0 {
		return Status{}, errFossilInfo
	}
	checkout := out[i+len(prefix):]
	i = strings.Index(checkout, suffix)
	if i < 0 {
		return Status{}, errFossilInfo
	}
	checkout = strings.TrimSpace(checkout[:i])

	i = strings.IndexByte(checkout, ' ')
	if i < 0 {
		return Status{}, errFossilInfo
	}
	rev := checkout[:i]

	commitTime, err := time.ParseInLocation("2006-01-02 15:04:05", checkout[i+1:], time.UTC)
	if err != nil {
		return Status{}, fmt.Errorf("%v: %v", errFossilInfo, err)
	}

	// Also look for untracked changes.
	outb, err = vcsFossil.runOutputVerboseOnly(rootDir, "changes --differ")
	if err != nil {
		return Status{}, err
	}
	uncommitted := len(outb) > 0

	return Status{
		Revision:    rev,
		CommitTime:  commitTime,
		Uncommitted: uncommitted,
	}, nil
}

func (v *Cmd) String() string {
	return v.Name
}

// run runs the command line cmd in the given directory.
// keyval is a list of key, value pairs. run expands
// instances of {key} in cmd into value, but only after
// splitting cmd into individual arguments.
// If an error occurs, run prints the command line and the
// command's combined stdout+stderr to standard error.
// Otherwise run discards the command's output.
func (v *Cmd) run(dir string, cmd string, keyval ...string) error {
	_, err := v.run1(dir, cmd, keyval, true)
	return err
}

// runVerboseOnly is like run but only generates error output to standard error in verbose mode.
func (v *Cmd) runVerboseOnly(dir string, cmd string, keyval ...string) error {
	_, err := v.run1(dir, cmd, keyval, false)
	return err
}

// runOutput is like run but returns the output of the command.
func (v *Cmd) runOutput(dir string, cmd string, keyval ...string) ([]byte, error) {
	return v.run1(dir, cmd, keyval, true)
}

// runOutputVerboseOnly is like runOutput but only generates error output to
// standard error in verbose mode.
func (v *Cmd) runOutputVerboseOnly(dir string, cmd string, keyval ...string) ([]byte, error) {
	return v.run1(dir, cmd, keyval, false)
}

// run1 is the generalized implementation of run and runOutput.
func (v *Cmd) run1(dir string, cmdline string, keyval []string, verbose bool) ([]byte, error) {
	m := make(map[string]string)
	for i := 0; i < len(keyval); i += 2 {
		m[keyval[i]] = keyval[i+1]
	}
	args := strings.Fields(cmdline)
	for i, arg := range args {
		args[i] = expand(m, arg)
	}

	if len(args) >= 2 && args[0] == "-go-internal-mkdir" {
		var err error
		if filepath.IsAbs(args[1]) {
			err = os.Mkdir(args[1], fs.ModePerm)
		} else {
			err = os.Mkdir(filepath.Join(dir, args[1]), fs.ModePerm)
		}
		if err != nil {
			return nil, err
		}
		args = args[2:]
	}

	if len(args) >= 2 && args[0] == "-go-internal-cd" {
		if filepath.IsAbs(args[1]) {
			dir = args[1]
		} else {
			dir = filepath.Join(dir, args[1])
		}
		args = args[2:]
	}

	_, err := exec.LookPath(v.Cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"go: missing %s command. See https://golang.org/s/gogetcmd\n",
			v.Name)
		return nil, err
	}

	cmd := exec.Command(v.Cmd, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "# cd %s; %s %s\n", dir, v.Cmd, strings.Join(args, " "))
			if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
				os.Stderr.Write(ee.Stderr)
			} else {
				fmt.Fprintln(os.Stderr, err.Error())
			}
		}
	}
	return out, err
}

// Create creates a new copy of repo in dir.
// The parent of dir must exist; dir must not.
func (v *Cmd) Create(dir, repo string) error {
	for _, cmd := range v.CreateCmd {
		if err := v.run(filepath.Dir(dir), cmd, "dir", dir, "repo", repo); err != nil {
			return err
		}
	}
	return nil
}

// Download downloads any new changes for the repo in dir.
func (v *Cmd) Download(dir string) error {
	for _, cmd := range v.DownloadCmd {
		if err := v.run(dir, cmd); err != nil {
			return err
		}
	}
	return nil
}

// Tags returns the list of available tags for the repo in dir.
func (v *Cmd) Tags(dir string) ([]string, error) {
	var tags []string
	for _, tc := range v.TagCmd {
		out, err := v.runOutput(dir, tc.cmd)
		if err != nil {
			return nil, err
		}
		re := regexp.MustCompile(`(?m-s)` + tc.pattern)
		for _, m := range re.FindAllStringSubmatch(string(out), -1) {
			tags = append(tags, m[1])
		}
	}
	return tags, nil
}

// tagSync syncs the repo in dir to the named tag,
// which either is a tag returned by tags or is v.tagDefault.
func (v *Cmd) TagSync(dir, tag string) error {
	if v.TagSyncCmd == nil {
		return nil
	}
	if tag != "" {
		for _, tc := range v.TagLookupCmd {
			out, err := v.runOutput(dir, tc.cmd, "tag", tag)
			if err != nil {
				return err
			}
			re := regexp.MustCompile(`(?m-s)` + tc.pattern)
			m := re.FindStringSubmatch(string(out))
			if len(m) > 1 {
				tag = m[1]
				break
			}
		}
	}

	if tag == "" && v.TagSyncDefault != nil {
		for _, cmd := range v.TagSyncDefault {
			if err := v.run(dir, cmd); err != nil {
				return err
			}
		}
		return nil
	}

	for _, cmd := range v.TagSyncCmd {
		if err := v.run(dir, cmd, "tag", tag); err != nil {
			return err
		}
	}
	return nil
}

// FromDir inspects dir and its parents to determine the
// version control system and code repository to use.
// If no repository is found, FromDir returns an error
// equivalent to os.ErrNotExist.
func FromDir(dir, srcRoot string, allowNesting bool) (repoDir string, vcsCmd *Cmd, err error) {
	// Clean and double-check that dir is in (a subdirectory of) srcRoot.
	dir = filepath.Clean(dir)
	if srcRoot != "" {
		srcRoot = filepath.Clean(srcRoot)
		if len(dir) <= len(srcRoot) || dir[len(srcRoot)] != filepath.Separator {
			return "", nil, fmt.Errorf("directory %q is outside source root %q", dir, srcRoot)
		}
	}

	origDir := dir
	for len(dir) > len(srcRoot) {
		for _, vcs := range vcsList {
			if _, err := statAny(dir, vcs.RootNames); err == nil {
				// Record first VCS we find.
				// If allowNesting is false (as it is in GOPATH), keep looking for
				// repositories in parent directories and report an error if one is
				// found to mitigate VCS injection attacks.
				if vcsCmd == nil {
					vcsCmd = vcs
					repoDir = dir
					if allowNesting {
						return repoDir, vcsCmd, nil
					}
					continue
				}
				// Allow .git inside .git, which can arise due to submodules.
				if vcsCmd == vcs && vcs.Cmd == "git" {
					continue
				}
				// Otherwise, we have one VCS inside a different VCS.
				return "", nil, fmt.Errorf("directory %q uses %s, but parent %q uses %s",
					repoDir, vcsCmd.Cmd, dir, vcs.Cmd)
			}
		}

		// Move to parent.
		ndir := filepath.Dir(dir)
		if len(ndir) >= len(dir) {
			break
		}
		dir = ndir
	}
	if vcsCmd == nil {
		return "", nil, &vcsNotFoundError{dir: origDir}
	}
	return repoDir, vcsCmd, nil
}

// statAny provides FileInfo for the first filename found in the directory.
// Otherwise, it returns the last error seen.
func statAny(dir string, filenames []string) (os.FileInfo, error) {
	if len(filenames) == 0 {
		return nil, errors.New("invalid argument: no filenames provided")
	}

	var err error
	var fi os.FileInfo
	for _, name := range filenames {
		fi, err = os.Stat(filepath.Join(dir, name))
		if err == nil {
			return fi, nil
		}
	}

	return nil, err
}

type vcsNotFoundError struct {
	dir string
}

func (e *vcsNotFoundError) Error() string {
	return fmt.Sprintf("directory %q is not using a known version control system", e.dir)
}

func (e *vcsNotFoundError) Is(err error) bool {
	return err == os.ErrNotExist
}

// RepoRoot describes the repository root for a tree of source code.
type RepoRoot struct {
	Repo     string // repository URL, including scheme
	Root     string // import path corresponding to root of repo
	IsCustom bool   // defined by served <meta> tags (as opposed to hard-coded pattern)
	VCS      *Cmd
}

// ModuleMode specifies whether to prefer modules when looking up code sources.
type ModuleMode int

// A ImportMismatchError is returned where metaImport/s are present
// but none match our import path.
type ImportMismatchError struct {
	importPath string
	mismatches []string // the meta imports that were discarded for not matching our importPath
}

func (m ImportMismatchError) Error() string {
	formattedStrings := make([]string, len(m.mismatches))
	for i, pre := range m.mismatches {
		formattedStrings[i] = fmt.Sprintf("meta tag %s did not match import path %s", pre, m.importPath)
	}
	return strings.Join(formattedStrings, ", ")
}

// expand rewrites s to replace {k} with match[k] for each key k in match.
func expand(match map[string]string, s string) string {
	// We want to replace each match exactly once, and the result of expansion
	// must not depend on the iteration order through the map.
	// A strings.Replacer has exactly the properties we're looking for.
	oldNew := make([]string, 0, 2*len(match))
	for k, v := range match {
		oldNew = append(oldNew, "{"+k+"}", v)
	}
	return strings.NewReplacer(oldNew...).Replace(s)
}
