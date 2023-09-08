package editors

import (
	"fmt"
	"net/url"
	"strings"
)

/*
This file was added by Encore and is not part of the original GitHub Desktop codebase
*/

func convertFilePathToURLScheme(editorName string, fullPath string, startLine int, startCol int) string {
	switch strings.ToLower(editorName) {
	case "vscode", "visual studio code", "visual studio code (insiders)":
		if startLine > 0 {
			fullPath = fmt.Sprintf("%s:%d", fullPath, startLine)
		}
		return toURLScheme("vscode", "file", fullPath, "", "", 0, 0)
	case "jetbrains goland", "goland":
		return toJetBrainsScheme("goland", fullPath, startLine, startCol)
	case "jetbrains phpstorm", "phpstorm":
		return toJetBrainsScheme("phpstorm", fullPath, startLine, startCol)
	case "jetbrains pycharm", "pycharm", "pycharm community edition":
		return toJetBrainsScheme("pycharm", fullPath, startLine, startCol)
	case "jetbrains rubymine", "rubymine":
		return toJetBrainsScheme("rubymine", fullPath, startLine, startCol)
	case "jetbrains webstorm", "webstorm":
		return toJetBrainsScheme("webstorm", fullPath, startLine, startCol)
	case "jetbrains intellij", "intellij", "idea", "intellij idea", "intellij idea community edition":
		return toJetBrainsScheme("idea", fullPath, startLine, startCol)
	case "jetbrains clion", "clion":
		return toJetBrainsScheme("clion", fullPath, startLine, startCol)
	case "textmate", "mate":
		return toOpenURLScheme("txmt", "", fullPath, startLine, startCol)
	case "bbedit":
		return toOpenURLScheme("bbedit", "", fullPath, startLine, startCol)
	default:
		return fullPath
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
