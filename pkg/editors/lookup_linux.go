//go:build linux

package editors

import (
	"context"
)

// LinuxExternalEditor represents an external editor on Linux
type LinuxExternalEditor struct {
	// Name of the editor. It will be used both as identifier and user-facing.
	Name EditorName

	// List of possible paths where the editor's executable might be located.
	Paths []string
}

// This list contains all the external editors supported on Linux. Add a new
// entry here to add support for your favorite editor.
var editors = []LinuxExternalEditor{
	{
		Name:  Atom,
		Paths: []string{"/snap/bin/atom", "/usr/bin/atom"},
	},
	{
		Name:  Neovim,
		Paths: []string{"/usr/bin/nvim"},
	},
	{
		Name:  NeovimQt,
		Paths: []string{"/usr/bin/nvim-qt"},
	},
	{
		Name:  Neovide,
		Paths: []string{"/usr/bin/neovide"},
	},
	{
		Name:  GVim,
		Paths: []string{"/usr/bin/gvim"},
	},
	{
		Name: VSCode,
		Paths: []string{
			"/usr/share/code/bin/code",
			"/snap/bin/code",
			"/usr/bin/code",
			"/mnt/c/Program Files/Microsoft VS Code/bin/code",
		},
	},
	{
		Name:  VSCodeInsiders,
		Paths: []string{"/snap/bin/code-insiders", "/usr/bin/code-insiders"},
	},
	{
		Name: VSCodium,
		Paths: []string{
			"/usr/bin/codium",
			"/var/lib/flatpak/app/com.vscodium.codium",
			"/usr/share/vscodium-bin/bin/codium",
		},
	},
	{
		Name:  SublimeText,
		Paths: []string{"/usr/bin/subl"},
	},
	{
		Name:  Typora,
		Paths: []string{"/usr/bin/typora"},
	},
	{
		Name: SlickEdit,
		Paths: []string{
			"/opt/slickedit-pro2018/bin/vs",
			"/opt/slickedit-pro2017/bin/vs",
			"/opt/slickedit-pro2016/bin/vs",
			"/opt/slickedit-pro2015/bin/vs",
		},
	},
	{
		// Code editor for elementary OS
		// https://github.com/elementary/code
		Name:  Code,
		Paths: []string{"/usr/bin/io.elementary.code"},
	},
	{
		Name:  LiteXL,
		Paths: []string{"/usr/bin/lite-xl"},
	},
	{
		Name: JetbrainsPhpStorm,
		Paths: []string{
			"/snap/bin/phpstorm",
			".local/share/JetBrains/Toolbox/scripts/phpstorm",
		},
	},
	{
		Name: JetbrainsGoLand,
		Paths: []string{
			"/snap/bin/goland",
			".local/share/JetBrains/Toolbox/scripts/goland",
		},
	},
	{
		Name: JetbrainsWebStorm,
		Paths: []string{
			"/snap/bin/webstorm",
			".local/share/JetBrains/Toolbox/scripts/webstorm",
		},
	},
	{
		Name:  JetbrainsIntelliJ,
		Paths: []string{"/snap/bin/idea", ".local/share/JetBrains/Toolbox/scripts/idea"},
	},
	{
		Name: JetbrainsPyCharm,
		Paths: []string{
			"/snap/bin/pycharm",
			".local/share/JetBrains/Toolbox/scripts/pycharm",
		},
	},
	{
		Name: Studio,
		Paths: []string{
			"/snap/bin/studio",
			".local/share/JetBrains/Toolbox/scripts/studio",
		},
	},
	{
		Name:  Emacs,
		Paths: []string{"/snap/bin/emacs", "/usr/local/bin/emacs", "/usr/bin/emacs"},
	},
	{
		Name:  Kate,
		Paths: []string{"/usr/bin/kate"},
	},
	{
		Name:  GEdit,
		Paths: []string{"/usr/bin/gedit"},
	},
	{
		Name:  GnomeTextEditor,
		Paths: []string{"/usr/bin/gnome-text-editor"},
	},
	{
		Name:  GnomeBuilder,
		Paths: []string{"/usr/bin/gnome-builder"},
	},
	{
		Name:  Notepadqq,
		Paths: []string{"/usr/bin/notepadqq"},
	},
	{
		Name:  Geany,
		Paths: []string{"/usr/bin/geany"},
	},
	{
		Name:  Mousepad,
		Paths: []string{"/usr/bin/mousepad"},
	},
}

// Returns the first available path from the provided list.
func getAvailablePath(paths []string) string {
	for _, path := range paths {
		if pathExists(path) {
			return path
		}
	}
	return ""
}

// Returns a list of available editors with their paths.
func getAvailableEditors(_ context.Context) ([]FoundEditor, error) {
	var results []FoundEditor
	for _, editor := range editors {
		path := getAvailablePath(editor.Paths) // Assuming the editor struct has a Paths field
		if path != "" {
			results = append(results, FoundEditor{Editor: editor.Name, Path: path})
		}
	}
	return results, nil
}
