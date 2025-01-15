package main

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"

	"encr.dev/internal/userconfig"
	"encr.dev/pkg/xos"
)

func main() {
	repoRoot := resolveRepoRoot()
	docsDir := filepath.Join(repoRoot, "docs")

	for _, lang := range []string{"go", "ts"} {
		docs := generateDocs(lang)
		dst := filepath.Join(docsDir, lang, "cli", "config-reference.md")
		if err := xos.WriteFile(dst, []byte(docs), 0644); err != nil {
			log.Fatalf("error writing %s docs file: %v\n", lang, err)
		}
	}
	log.Printf("successfully regenerated docs")
}

func generateDocs(lang string) string {
	return docsHeader(lang) + "\n" + userconfig.MarkdownDocs()
}

func docsHeader(lang string) string {
	return fmt.Sprintf(`---
seotitle: Encore CLI Configuration Options
seodesc: Configuration options to customize the behavior of the Encore CLI.
title: Configuration Reference
subtitle: Configuration options to customize the behavior of the Encore CLI.
lang: %s
---
`, lang)
}

func resolveRepoRoot() string {
	// Use `git rev-parse --show-toplevel` to get the root of the repository
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error running git rev-parse: %v\n", err)
	}
	return filepath.Clean(strings.TrimSpace(string(out)))
}
