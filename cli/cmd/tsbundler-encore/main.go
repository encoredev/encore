package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"

	"encr.dev/internal/version"

	flag "github.com/spf13/pflag"
)

var (
	entryPoints      []string
	specifiedEngines []string
	// replacementFile  string
	outDir   string
	bundle   bool
	minify   bool
	help     bool
	logLevel int
)

// main is the entry point for the tsbundler-encore command.
//
// It is responsible for parsing the command line flags, validating the input, and then triggering esbuild.
//
// Run with --help for more information.
func main() {
	// Required flags
	// flag.StringVar(&replacementFile, "replacements", "", "Replacement file or json object (default read from stdin)")

	// Optional flags
	flag.StringVar(&outDir, "outdir", "dist", "Output directory")
	flag.BoolVar(&bundle, "bundle", true, "Bundle all dependencies")
	flag.BoolVar(&minify, "minify", false, "Minify output (default false)")
	flag.StringArrayVar(&specifiedEngines, "engine", []string{"node:21"}, "Target engine")
	flag.CountVarP(&logLevel, "verbose", "v", "Increase logging level (can be specified multiple times)")
	flag.BoolVarP(&help, "help", "h", false, "Print help")
	flag.Usage = printHelp
	flag.Parse()

	entryPoints = flag.Args()
	if help {
		printHelp()
		os.Exit(0)
	}

	// Validate input (note: these functions will exit on error)
	validateEntrypointParams()
	engines := readEngines()
	// replacements := readReplacementMapping()

	// Create our transformer plugin
	// rewritePlugin := api.Plugin{
	// 	Name: "encore-codegen-transformer",
	// 	Setup: func(build api.PluginBuild) {
	// 		build.OnLoad(
	// 			api.OnLoadOptions{Filter: `\.(ts|js)(x?)$`},
	// 			func(args api.OnLoadArgs) (api.OnLoadResult, error) {
	// 				replacement, found := replacements[args.Path]
	// 				if !found {
	// 					return api.OnLoadResult{}, nil
	// 				}

	// 				contentsBytes, err := os.ReadFile(replacement)
	// 				if err != nil {
	// 					return api.OnLoadResult{}, fmt.Errorf("error reading replacement file: %w", err)
	// 				}
	// 				content := string(contentsBytes)

	// 				return api.OnLoadResult{
	// 					PluginName: "encore-codegen-transformer",
	// 					Contents:   &content,
	// 					Loader:     api.LoaderTS,
	// 				}, nil
	// 			},
	// 		)
	// 	},
	// }

	banner := `// This file was bundled by Encore ` + version.Version + `
//
// https://encore.dev`

	outBase := ""
	if len(entryPoints) == 1 {
		// If there's a single entrypoint, use its directory as the outbase
		// as otherwise esbuild won't include the "[dir]" token in the output.
		outBase = filepath.Dir(filepath.Dir(entryPoints[0]))
	}

	// Trigger esbuild
	result := api.Build(api.BuildOptions{
		// Setup base settings
		LogLevel:  api.LogLevelWarning - api.LogLevel(logLevel),
		Banner:    map[string]string{"js": banner},
		Charset:   api.CharsetUTF8,
		Sourcemap: api.SourceMapExternal,
		Packages:  api.PackagesExternal,
		Plugins:   []api.Plugin{
			// rewritePlugin,
		},

		// Set our build target
		Platform: api.PlatformNode,
		Format:   api.FormatESModule,
		Target:   api.ES2022,
		Engines:  engines,

		// Minification settings
		MinifyWhitespace:  minify,
		MinifySyntax:      minify,
		MinifyIdentifiers: minify,

		// Pass in what we want to build
		EntryNames:  "[dir]/[name]",
		EntryPoints: entryPoints,
		Bundle:      bundle,
		Outdir:      outDir,
		Outbase:     outBase,
		Write:       true, // Write to outdir
		OutExtension: map[string]string{
			".js": ".mjs",
		},
	})

	if len(result.Errors) > 0 {
		os.Exit(1)
	}
}

func printHelp() {
	binary := filepath.Base(os.Args[0])

	// Base usage help
	versionStr := fmt.Sprintf("tsbundler-encore (%s)", version.Version)
	_, _ = fmt.Fprintf(os.Stderr, "%s\n%s\n", versionStr, strings.Repeat("=", len(versionStr)))
	_, _ = fmt.Fprintf(os.Stderr, "\nUsage: %s <entry point(s)...> [options]\n", binary)
	flag.PrintDefaults()

	// Replacements help
	// _, _ = fmt.Fprintf(os.Stderr, "\nReplacements JSON Format:\n")
	// _, _ = fmt.Fprintf(os.Stderr, "      {\n")
	// _, _ = fmt.Fprintf(os.Stderr, "        \"/absolute/path/to/file.ts\": \"/path/to/replacement.ts\",\n")
	// _, _ = fmt.Fprintf(os.Stderr, "        \"/absolute/path/to/file2.ts\": \"/path/to/replacement2.ts\"\n")
	// _, _ = fmt.Fprintf(os.Stderr, "      }\n")

	// Engine help
	_, _ = fmt.Fprintf(os.Stderr, "\nEngines:\n\nEngines can be specified as a name, or a name and version separated by a colon,\nfor example \"node:21\" or \"node\". Multiple engines can be specified if required.\n\nThe supported engines are:\n")
	_, _ = fmt.Fprintf(os.Stderr, "      - node\n")
	_, _ = fmt.Fprintf(os.Stderr, "      - bun\n")
	_, _ = fmt.Fprintf(os.Stderr, "      - deno\n")
	_, _ = fmt.Fprintf(os.Stderr, "      - rhino\n")
}

// validateEntrypointParams validates that the entry points parameters was specified and that all entry points exist
// and are readable on the file system.
func validateEntrypointParams() {
	if len(entryPoints) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Error: at least one entry point must be specified\n\n")
		printHelp()
		os.Exit(1)
	}

	for _, entryPoint := range entryPoints {
		if st, err := os.Stat(entryPoint); errors.Is(err, fs.ErrNotExist) {
			_, _ = fmt.Fprintf(os.Stderr, "Error: entry point %s does not exist\n", entryPoint)
			os.Exit(1)
		} else if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: error reading entry point %s: %s\n", entryPoint, err)
			os.Exit(1)
		} else if st.IsDir() {
			_, _ = fmt.Fprintf(os.Stderr, "Error: entry point %s is a directory\n", entryPoint)
			os.Exit(1)
		}
	}
}

// readReplacementMapping reads a replacement mapping from either a file or stdin depending
// on if the replacementFile flag was specified.
//
// It then validates that all the keys are valid paths to files and the values are valid paths to files.
// func readReplacementMapping() map[string]string {
// 	out := make(map[string]string)

// 	// If a replacement file was specified, read it
// 	replacementFile = strings.TrimSpace(replacementFile)
// 	if replacementFile != "" {
// 		if replacementFile[0] == '{' {
// 			err := json.Unmarshal([]byte(replacementFile), &out)
// 			if err != nil {
// 				_, _ = fmt.Fprintf(os.Stderr, "Error parsing replacement object: %s\n", err)
// 				os.Exit(1)
// 			}
// 		} else {
// 			data, err := os.ReadFile(replacementFile)
// 			if err != nil {
// 				_, _ = fmt.Fprintf(os.Stderr, "Error reading replacement file: %s\n", err)
// 				os.Exit(1)
// 			}

// 			err = json.Unmarshal(data, &out)
// 			if err != nil {
// 				_, _ = fmt.Fprintf(os.Stderr, "Error parsing replacement file: %s\n", err)
// 				os.Exit(1)
// 			}
// 		}
// 	} else {
// 		// Check something is being piped in
// 		info, _ := os.Stdin.Stat()
// 		if (info.Mode()&os.ModeCharDevice) != 0 || info.Size() <= 0 {
// 			_, _ = fmt.Fprintf(os.Stderr, "Error: no replacement file specified and nothing piped in\n")
// 			os.Exit(1)
// 		}

// 		// Otherwise, read from stdin
// 		if err := json.NewDecoder(os.Stdin).Decode(&out); err != nil {
// 			_, _ = fmt.Fprintf(os.Stderr, "Error reading replacement file from stdin: %s\n", err)
// 			os.Exit(1)
// 		}
// 	}

// 	// Validate that all the keys are valid paths to files and the values are valid paths to files
// 	for key, value := range out {
// 		// Validate key
// 		if st, err := os.Stat(key); errors.Is(err, fs.ErrNotExist) {
// 			_, _ = fmt.Fprintf(os.Stderr, "Error: replacement key %s does not exist\n", key)
// 			os.Exit(1)
// 		} else if err != nil {
// 			_, _ = fmt.Fprintf(os.Stderr, "Error: error reading replacement key %s: %s\n", key, err)
// 			os.Exit(1)
// 		} else if st.IsDir() {
// 			_, _ = fmt.Fprintf(os.Stderr, "Error: replacement key %s is a directory\n", key)
// 			os.Exit(1)
// 		} else if !filepath.IsAbs(key) {
// 			_, _ = fmt.Fprintf(os.Stderr, "Error: replacement key %s is not an absolute path\n", key)
// 			os.Exit(1)
// 		}

// 		// Validate value
// 		if st, err := os.Stat(value); errors.Is(err, fs.ErrNotExist) {
// 			_, _ = fmt.Fprintf(os.Stderr, "Error: replacement value %s does not exist\n", value)
// 			os.Exit(1)
// 		} else if err != nil {
// 			_, _ = fmt.Fprintf(os.Stderr, "Error: error reading replacement value %s: %s\n", value, err)
// 			os.Exit(1)
// 		} else if st.IsDir() {
// 			_, _ = fmt.Fprintf(os.Stderr, "Error: replacement value %s is a directory\n", value)
// 			os.Exit(1)
// 		}
// 	}

// 	return out
// }

// readEngines reads the engines from the specified flag and returns a list of engines.
func readEngines() []api.Engine {
	if len(specifiedEngines) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Error: at least one engine must be specified\n\n")
		printHelp()
		os.Exit(1)
	}

	var engines []api.Engine
	for _, engineName := range specifiedEngines {
		engineName = strings.ToLower(strings.TrimSpace(engineName))
		engineName, engineVersion, _ := strings.Cut(engineName, ":")

		var eng api.Engine
		switch engineName {
		case "node", "bun": // Note: esbuild doesn't have a "bun" engine (yet), but to future proof we'll alias it to node
			eng = api.Engine{Name: api.EngineNode, Version: engineVersion}
		case "deno":
			eng = api.Engine{Name: api.EngineDeno, Version: engineVersion}
		case "rhino":
			eng = api.Engine{Name: api.EngineRhino, Version: engineVersion}
		default:
			_, _ = fmt.Fprintf(os.Stderr, "Error: unknown/unsupported engine %s\n\n", engineName)
			printHelp()
			os.Exit(1)
		}

		engines = append(engines, eng)
	}

	return engines
}
