package cmdutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/fatih/color"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc/status"

	"encr.dev/cli/internal/manifest"
	"encr.dev/pkg/appfile"
	"encr.dev/pkg/errinsrc"
	"encr.dev/pkg/errlist"
)

var (
	ErrNoEncoreApp    = errors.New("no encore.app found in directory (or any of the parent directories)")
	ErrEncoreAppIsDir = errors.New("encore.app is a directory, not a file")
)

// MaybeAppRoot determines the app root by looking for the "encore.app" file,
// initially in the current directory and then recursively in parent directories
// up to the filesystem root.
//
// It reports the absolute path to the app root, and the
// relative path from the app root to the working directory.
func MaybeAppRoot() (appRoot, relPath string, err error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	return FindAppRootFromDir(dir)
}

func FindAppRootFromDir(dir string) (appRoot, relPath string, err error) {
	rel := "."
	for {
		path := filepath.Join(dir, "encore.app")
		fi, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			dir2 := filepath.Dir(dir)
			if dir2 == dir {
				return "", "", ErrNoEncoreApp
			}
			rel = filepath.Join(filepath.Base(dir), rel)
			dir = dir2
			continue
		} else if err != nil {
			return "", "", err
		} else if fi.IsDir() {
			return "", "", ErrEncoreAppIsDir
		} else {
			return dir, rel, nil
		}
	}
}

// AppRoot is like MaybeAppRoot but instead of returning an error
// it prints it to stderr and exits.
func AppRoot() (appRoot, relPath string) {
	appRoot, relPath, err := MaybeAppRoot()
	if err != nil {
		Fatal(err)
	}
	return appRoot, relPath
}

// WorkspaceRoot determines the workspace root by looking for the .git folder in app root or parents to it.
// It reports the absolute path to the workspace root.
func WorkspaceRoot(appRoot string) string {
	dir := appRoot
	for {
		path := filepath.Join(dir, ".git")
		fi, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			dir2 := filepath.Dir(dir)
			if dir2 == dir {
				return appRoot
			}
			dir = dir2
			continue
		} else if err != nil {
			Fatal(err)
		} else if !fi.IsDir() {
			continue
		} else {
			return dir
		}
	}
}

func AppSlugOrLocalID() string {
	appRoot, _ := AppRoot()
	appID, _ := appfile.Slug(appRoot)
	if appID == "" {
		mf, err := manifest.ReadOrCreate(appRoot)
		if err != nil {
			Fatalf("failed to read app manifest: %v", err)
		}
		appID = mf.LocalID
	}
	return appID
}

// AppSlug reports the current app's app slug.
// It throws a fatal error if the app is not connected with the Encore Platform.
func AppSlug() string {
	appRoot, _ := AppRoot()
	appSlug, err := appfile.Slug(appRoot)
	if err != nil {
		Fatal(err)
	} else if appSlug == "" {
		Fatal("app is not linked with the Encore Platform (see 'encore app link')")
	}
	return appSlug
}

func Fatal(args ...any) {
	// Prettify gRPC errors
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			if s, ok := status.FromError(err); ok {
				args[i] = s.Message()
			}
		}
	}

	red := color.New(color.FgRed)
	_, _ = red.Fprint(os.Stderr, "error: ")
	_, _ = red.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

func Fatalf(format string, args ...any) {
	// Prettify gRPC errors
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			if s, ok := status.FromError(err); ok {
				args[i] = s.Message()
			}
		}
	}

	Fatal(fmt.Sprintf(format, args...))
}

func DisplayError(out *os.File, err []byte) {
	if len(err) == 0 {
		return
	}

	// Get the width of the terminal we're rendering in
	// if we can so we render using the most space possible.
	width, _, sizeErr := terminal.GetSize(int(out.Fd()))
	if sizeErr == nil {
		errinsrc.TerminalWidth = width
	}

	// Unmarshal the error into a structured errlist
	errList := errlist.New(nil)
	if err := json.Unmarshal(err, &errList); err != nil {
		Fatalf("unable to parse error: %v", err)
	}

	if errList.Len() == 0 {
		return
	}

	_, _ = os.Stderr.Write([]byte(errList.Error()))
}

var Newline string

func init() {
	switch runtime.GOOS {
	case "windows":
		Newline = "\r\n"
	default:
		Newline = "\n"
	}
}
