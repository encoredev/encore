//go:build darwin

package editors

import (
	"bytes"
	"context"
	"strings"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/errgroup"
	exec "golang.org/x/sys/execabs"
)

// DarwinExternalEditor represents an external editor on macOS
type DarwinExternalEditor struct {
	// Name of the editor. It will be used both as identifier and user-facing.
	Name string

	// List of bundle identifiers that are used by the app in its multiple versions.
	BundleIdentifiers []string
}

// This list contains all the external editors supported on macOS. Add a new
// entry here to add support for your favorite editor.
var editors = []DarwinExternalEditor{
	{
		Name:              "Atom",
		BundleIdentifiers: []string{"com.github.atom"},
	},
	{
		Name:              "Aptana Studio",
		BundleIdentifiers: []string{"aptana.studio"},
	},
	{
		Name:              "MacVim",
		BundleIdentifiers: []string{"org.vim.MacVim"},
	},
	{
		Name:              "Neovide",
		BundleIdentifiers: []string{"com.neovide.neovide"},
	},
	{
		Name:              "VimR",
		BundleIdentifiers: []string{"com.qvacua.VimR"},
	},
	{
		Name:              "Visual Studio Code",
		BundleIdentifiers: []string{"com.microsoft.VSCode"},
	},
	{
		Name:              "Visual Studio Code (Insiders)",
		BundleIdentifiers: []string{"com.microsoft.VSCodeInsiders"},
	},
	{
		Name:              "VSCodium",
		BundleIdentifiers: []string{"com.visualstudio.code.oss", "com.vscodium"},
	},
	{
		Name: "Sublime Text",
		BundleIdentifiers: []string{
			"com.sublimetext.4",
			"com.sublimetext.3",
			"com.sublimetext.2",
		},
	},
	{
		Name:              "BBEdit",
		BundleIdentifiers: []string{"com.barebones.bbedit"},
	},
	{
		Name:              "PhpStorm",
		BundleIdentifiers: []string{"com.jetbrains.PhpStorm"},
	},
	{
		Name:              "PyCharm",
		BundleIdentifiers: []string{"com.jetbrains.PyCharm"},
	},
	{
		Name:              "PyCharm Community Edition",
		BundleIdentifiers: []string{"com.jetbrains.pycharm.ce"},
	},
	{
		Name:              "DataSpell",
		BundleIdentifiers: []string{"com.jetbrains.DataSpell"},
	},
	{
		Name:              "RubyMine",
		BundleIdentifiers: []string{"com.jetbrains.RubyMine"},
	},
	{
		Name:              "RStudio",
		BundleIdentifiers: []string{"org.rstudio.RStudio", "com.rstudio.desktop"},
	},
	{
		Name:              "TextMate",
		BundleIdentifiers: []string{"com.macromates.TextMate"},
	},
	{
		Name:              "Brackets",
		BundleIdentifiers: []string{"io.brackets.appshell"},
	},
	{
		Name:              "WebStorm",
		BundleIdentifiers: []string{"com.jetbrains.WebStorm"},
	},
	{
		Name:              "CLion",
		BundleIdentifiers: []string{"com.jetbrains.CLion"},
	},
	{
		Name:              "Typora",
		BundleIdentifiers: []string{"abnerworks.Typora"},
	},
	{
		Name:              "CodeRunner",
		BundleIdentifiers: []string{"com.krill.CodeRunner"},
	},
	{
		Name: "SlickEdit",
		BundleIdentifiers: []string{
			"com.slickedit.SlickEditPro2018",
			"com.slickedit.SlickEditPro2017",
			"com.slickedit.SlickEditPro2016",
			"com.slickedit.SlickEditPro2015",
		},
	},
	{
		Name:              "IntelliJ",
		BundleIdentifiers: []string{"com.jetbrains.intellij"},
	},
	{
		Name:              "IntelliJ Community Edition",
		BundleIdentifiers: []string{"com.jetbrains.intellij.ce"},
	},
	{
		Name:              "Xcode",
		BundleIdentifiers: []string{"com.apple.dt.Xcode"},
	},
	{
		Name:              "GoLand",
		BundleIdentifiers: []string{"com.jetbrains.goland"},
	},
	{
		Name:              "Android Studio",
		BundleIdentifiers: []string{"com.google.android.studio"},
	},
	{
		Name:              "Rider",
		BundleIdentifiers: []string{"com.jetbrains.rider"},
	},
	{
		Name:              "Nova",
		BundleIdentifiers: []string{"com.panic.Nova"},
	},
	{
		Name:              "Emacs",
		BundleIdentifiers: []string{"org.gnu.Emacs"},
	},
	{
		Name:              "Lite XL",
		BundleIdentifiers: []string{"com.lite-xl"},
	},
	{
		Name:              "Fleet",
		BundleIdentifiers: []string{"Fleet.app"},
	},
	{
		Name:              "Pulsar",
		BundleIdentifiers: []string{"dev.pulsar-edit.pulsar"},
	},
	{
		Name:              "Zed",
		BundleIdentifiers: []string{"dev.zed.Zed"},
	},
	{
		Name:              "Zed (Preview)",
		BundleIdentifiers: []string{"dev.zed.Zed-Preview"},
	},
}

func findApplication(ctx context.Context, editor DarwinExternalEditor, foundEditors chan FoundEditor) error {
	for _, bundleIdentifier := range editor.BundleIdentifiers {
		path, err := getAppLocationByBundleID(ctx, bundleIdentifier)

		switch {
		case err != nil:
			return errors.WithStack(err)

		case path != "":
			foundEditors <- FoundEditor{
				Editor: editor.Name,
				Path:   path,
			}
		}
	}

	return nil
}

// getAppLocationByBundleID returns the location of the app with the given bundle identifier.
func getAppLocationByBundleID(ctx context.Context, bundleID string) (string, error) {
	cmd := exec.CommandContext(ctx, "mdfind", "kMDItemCFBundleIdentifier == '"+bundleID+"'")
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	// mdfind can return multiple results, so we'll just take the first one.
	results := strings.Split(out.String(), "\n")
	if len(results) > 0 {
		return results[0], nil
	}

	return "", nil
}

// Resolve a list of installed editors on the user's machine, using the known
// install identifiers that each OS supports.
func getAvailableEditors(ctx context.Context) ([]FoundEditor, error) {
	results := make([]FoundEditor, 0)

	grp, ctx := errgroup.WithContext(ctx)

	foundEditors := make(chan FoundEditor)
	errs := make(chan error, 1)
	for _, editor := range editors {
		editor := editor
		grp.Go(func() error {
			return findApplication(ctx, editor, foundEditors)
		})
	}

	go func() {
		errs <- grp.Wait()
		close(foundEditors)
	}()

	// Collect results and the error from the group
	for editor := range foundEditors {
		results = append(results, editor)
	}
	if err := <-errs; err != nil {
		return nil, errors.WithStack(err)
	}

	return results, nil
}
