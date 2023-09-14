package editors

/*
   This file was added by Encore and is not part of the original GitHub Desktop codebase
*/

import (
	"fmt"
	"net/url"
)

// convertFilePathToURLScheme converts a file path to a URL scheme that can be used to open to a specific
// line and column number.
//
// If the returned string should be executed as a URL, true is returned as the second argument. If the returned string
// should be executed as a normal argument against the editor, then false is returned
//
// If no URL scheme exists for that editor and empty string and false is returned.
func convertFilePathToURLScheme(editorName EditorName, fullPath string, startLine int, startCol int) (openArg string, executeAsURL bool) {
	switch editorName {
	case VSCode, VSCodeInsiders:
		if startLine > 0 {
			fullPath = fmt.Sprintf("%s:%d", fullPath, startLine)
		}
		return toURLScheme("vscode", "file", fullPath, "", "", 0, 0), true
	case JetbrainsGoLand:
		return toJetBrainsScheme("goland", fullPath, startLine, startCol), true
	case JetbrainsPhpStorm:
		return toJetBrainsScheme("phpstorm", fullPath, startLine, startCol), true
	case JetbrainsPyCharm, JetbrainsPyCharmCE:
		return toJetBrainsScheme("pycharm", fullPath, startLine, startCol), true
	case JetbrainsRubyMine:
		return toJetBrainsScheme("rubymine", fullPath, startLine, startCol), true
	case JetbrainsWebStorm:
		return toJetBrainsScheme("webstorm", fullPath, startLine, startCol), true
	case JetbrainsIntelliJ, JetbrainsIntelliJCE:
		return toJetBrainsScheme("idea", fullPath, startLine, startCol), true
	case JetbrainsCLion:
		return toJetBrainsScheme("clion", fullPath, startLine, startCol), true
	case TextMate:
		return toOpenURLScheme("txmt", "", fullPath, startLine, startCol), true
	case BBEdit:
		return toOpenURLScheme("bbedit", "", fullPath, startLine, startCol), true
	default:
		return "", false
	}
}

func toJetBrainsScheme(scheme string, file string, line int, col int) string {
	return toURLScheme(scheme, "open", "", "file", file, line, col)
}

func toOpenURLScheme(scheme string, basePath string, file string, line int, col int) string {
	return toURLScheme(scheme, "open", basePath, "url", fmt.Sprintf("file://%s", file), line, col)
}

func toURLScheme(scheme string, host string, basePath string, fileKey string, file string, line int, col int) string {
	u := &url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   basePath,
	}

	q := u.Query()
	if fileKey != "" && file != "" {
		q.Set(fileKey, file)
	}
	if line > 0 {
		q.Set("line", fmt.Sprintf("%d", line))

		if col > 0 {
			q.Set("col", fmt.Sprintf("%d", col))
		}
	}

	u.RawQuery = q.Encode()

	return u.String()
}
