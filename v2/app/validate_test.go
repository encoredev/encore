package app

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	goregexp "regexp"
	"strings"
	"testing"

	"cuelang.org/go/pkg/regexp"
	"github.com/pkg/diff"
	"github.com/rogpeppe/go-internal/testscript"

	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/option"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/testutil"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/config"
	"encr.dev/v2/parser/infra/cron"
	"encr.dev/v2/parser/infra/metrics"
	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/resource/usage"
)

var goldenUpdate = flag.Bool("golden-update", false, "update golden files")

func TestValidation(t *testing.T) {
	type testCfg struct {
		ignoreOutputCommand bool
	}
	t.Parallel()

	update := false
	if goldenUpdate != nil && *goldenUpdate {
		update = true
	}

	sourceDir := "../../parser/testdata"

	testscript.Run(t, testscript.Params{
		Dir:           sourceDir,
		UpdateScripts: update,
		Setup: func(env *testscript.Env) error {
			if err := testutil.TestScriptSetupFunc(env); err != nil {
				return err
			}
			env.Values["stderr"] = &bytes.Buffer{}
			env.Values["stdout"] = &bytes.Buffer{}
			env.Values["cfg"] = &testCfg{}
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			// The "parse" command runs the parser on the given testscript
			// and reports a failure if there are any errors.
			//
			// use like:
			//  - `parse`
			//  - `! parse` (if you want to check that there are errors)
			"parse": func(ts *testscript.TestScript, neg bool, args []string) {
				stdout := ts.Value("stdout").(*bytes.Buffer)
				printf := func(format string, args ...interface{}) {
					stdout.WriteString(fmt.Sprintf(format, args...) + "\n")
				}
				stderr := ts.Value("stderr").(*bytes.Buffer)
				defer func() {
					if err := recover(); err != nil {
						if l, ok := perr.IsBailout(err); ok {
							ts.Fatalf("bailout: %v", l.FormatErrors())
						} else {
							// We convert to an srcerrors error so that we can capture the stack
							e := srcerrors.UnhandledPanic(err)
							ts.Fatalf("panic: %v", e)
						}
					}

					ts.Logf("stdout: %s", stdout.String())
					ts.Logf("stderr: %s", stderr.String())
				}()

				// Setup the parse context
				tc := testutil.NewContextForTestScript(ts, false)
				tc.GoModTidy()
				tc.GoModDownload()
				p := parser.NewParser(tc.Context)

				// Parse the testscript
				parseResult := p.Parse()

				// ValidateAndDescribe the testscript
				desc := ValidateAndDescribe(tc.Context, parseResult)

				// If we're expecting parse errors, assert that we have them
				if neg {
					assertGoldenErrors(ts, tc.Errs, sourceDir, update)
				}

				// If we have errors, and we didn't expect them, fail the test
				// If we have no errors, and we expected them, fail the test
				// Otherwise write any errors to stderr so they can be asserted on
				if tc.Errs.Len() > 0 {
					if !neg {
						ts.Fatalf("unexpected errors: %s", tc.Errs.FormatErrors())
					}

					stderr.WriteString(tc.Errs.FormatErrors())
				} else if tc.Errs.Len() == 0 && neg {
					ts.Fatalf("expected errors, but none found")
				}

				// Now writeto stdout the description of the parsed app
				for _, svc := range desc.Services {
					if svc.Name != "fakesvcfortest" {
						printf("svc %s dbs=%s", svc.Name, "") // FIXME: bring databases in
					}
				}

				for _, svc := range desc.Services {
					if svc.Name == "fakesvcfortest" {
						// this service only exists to suppress the "no services found error"
						continue
					}

					svc.Framework.ForAll(func(fw *apiframework.ServiceDesc) {

						for _, rpc := range fw.Endpoints {
							if rpc == nil {
								ts.Fatalf("rpc is nil")
							}
							recvName := option.Map(rpc.Recv, func(recv *schema.Receiver) string {
								switch t := recv.Type.(type) {
								case *schema.NamedType:
									return "*" + t.Decl().Name
								case schema.NamedType:
									return "*" + t.Decl().Name
								case *schema.PointerType:
									return "*" + t.Elem.(*schema.NamedType).Decl().Name
								case schema.PointerType:
									return "*" + t.Elem.(schema.NamedType).Decl().Name
								default:
									panic(fmt.Sprintf("a reciver should only be a named type or pointer type: got %T", t))
								}
							}).GetOrElse("")

							printf("rpc %s.%s access=%v raw=%v path=%v recv=%v",
								svc.Name, rpc.Name, rpc.Access, rpc.Raw, rpc.Path, recvName,
							)
						}
					})
				}

				// First find all the bindings for each topic
				topicsByName := make(map[pkginfo.QualifiedName]*pubsub.Topic)
				for _, res := range desc.Parse.Resources() {
					switch res := res.(type) {
					case *pubsub.Topic:
						for _, b := range desc.Parse.PkgDeclBinds(res) {
							topicsByName[b.QualifiedName()] = res
						}
					}
				}

				for _, res := range desc.Parse.Resources() {
					switch res := res.(type) {
					case *config.Load:
						svc, found := desc.ServiceForPath(res.File.FSPath)
						if !found {
							ts.Fatalf("could not find service for path %s", res.File.FSPath)
						}
						printf("config %s %s", svc.Name, res.Type)
					case *cron.Job:
						printf("cronJob %s title=%q", res.Name, res.Title)
					case *pubsub.Topic:
						printf("pubsubTopic %s", res.Name)

						for _, u := range desc.Parse.Usages(res) {
							if mc, ok := u.(*usage.MethodCall); ok && mc.Method == "Publish" {
								if svc, found := desc.ServiceForPath(mc.File.FSPath); found {
									printf("pubsubPublisher %s %s\n", res.Name, svc.Name)
								} else {
									// TODO handle PubSub publishing from within global middleware
									ts.Fatalf("PubSub publishing outside of service NOT IMPLEMENTED")
								}
							}
						}
					case *pubsub.Subscription:
						svc, found := desc.ServiceForPath(res.File.FSPath)
						if !found {
							ts.Fatalf("could not find service for path %s", res.File.FSPath)
						}
						printf("pubsubSubscriber %s %s %s %d %d %d %d %d",
							topicsByName[res.Topic].Name, res.Name, svc.Name, res.Cfg.AckDeadline,
							res.Cfg.MessageRetention, res.Cfg.MaxRetries, res.Cfg.MinRetryBackoff,
							res.Cfg.MaxRetryBackoff)
					case *metrics.Metric:
						// TODO: implement this at the parser as we don't currently take the metric kind or labels in the same way
						// printf("metric %s %s %s %s\n", res.Name, res.ValueType, res.Kind, res.Labels)
					}
				}
			},

			// expectOut is a command that checks the stdout output contains the
			// given regex.
			//
			// It unique to the v2 parser as that has different handling of types, as such
			// if it is used in a testscript, we ignore any following calls to "output"
			"expectOut": func(ts *testscript.TestScript, neg bool, args []string) {
				ts.Value("cfg").(*testCfg).ignoreOutputCommand = true

				stdout := ts.Value("stdout").(*bytes.Buffer)
				m, err := regexp.Match(args[0], stdout.String())
				if err != nil {
					ts.Fatalf("invalid pattern: %v", err)
				}
				if !m && !neg {
					ts.Fatalf("output does not match %q", args[0])
				} else if m && neg {
					ts.Fatalf("output unexpectedly matches %q", args[0])
				}
			},

			// The "output" command checks that the output into stdout that we've collected
			// contains the given regex
			"output": func(ts *testscript.TestScript, neg bool, args []string) {
				if ts.Value("cfg").(*testCfg).ignoreOutputCommand {
					// "expectOut" was called, so we ignore this command
					return
				}

				stdout := ts.Value("stdout").(*bytes.Buffer)
				m, err := regexp.Match(args[0], stdout.String())
				if err != nil {
					ts.Fatalf("invalid pattern: %v", err)
				}
				if !m && !neg {
					ts.Fatalf("output does not match %q", args[0])
				} else if m && neg {
					ts.Fatalf("output unexpectedly matches %q", args[0])
				}
			},

			// The "Err" command is a no-op in the v2 parser, as we used expected errors
			// inside the test files to assert the full error message
			"err": func(ts *testscript.TestScript, neg bool, args []string) {},
		},
	})
}

func assertGoldenErrors(ts *testscript.TestScript, errs *perr.List, sourceDir string, updateGoldenFiles bool) {
	// Read the want: errors file
	// allow for it not to exist
	wantFile := ts.MkAbs("want: errors")
	data, err := os.ReadFile(wantFile)
	var wantErrors string
	if err == nil {
		wantErrors = string(data)
	}

	// Build up the "got errors string"
	var b strings.Builder
	errs.MakeRelative(ts.Getenv("WORK"), "")
	for i := 0; i < errs.Len(); i++ {
		err := *errs.At(i) // Copy the error so we can modify it

		// Remove the stack for the error, as it will change whenever the parser
		// changes, and that's not what we're testing for
		err.Stack = nil

		// Remove the code for the error, as it will change whenever the parser
		// has new errors introduced
		err.Params.Code = 9999

		if i != 0 {
			b.WriteString("\n\n")
		}

		b.WriteString(err.Error())
	}
	gotErrors := b.String()

	// Remove all trailing whitespace for every line
	gotErrors = goregexp.MustCompile(`(?m)[ \t]+$`).ReplaceAllString(gotErrors, "")

	// Ensure there is a single trailing newline
	gotErrors = strings.TrimSpace(gotErrors)
	if gotErrors != "" {
		gotErrors = "\n" + gotErrors + "\n"
	}

	// The two errors are the same, so we can return
	if wantErrors == gotErrors {
		return
	}

	// If we're updating the golden files, then write the new file
	// and don't fail the test
	if updateGoldenFiles {
		testutil.UpdateArchiveFile(ts, sourceDir, "want: errors", gotErrors)
		return
	}

	// pkg/diff is quadratic at the moment.
	// If the product of the number of lines in the inputs is too large,
	// don't call pkg.Diff at all as it might take tons of memory or time.
	// We found one million to be reasonable for an average laptop.
	const maxLineDiff = 1_000_000
	if strings.Count(wantErrors, "\n")*strings.Count(gotErrors, "\n") > maxLineDiff {
		ts.Fatalf("errors differ (two large to diff)")
		return
	}

	var sb strings.Builder
	if err := diff.Text("want: errors", "got: errors", wantErrors, gotErrors, &sb); err != nil {
		ts.Check(err)
	}

	ts.Logf("%s", sb.String())
	ts.Fatalf("wanted errors differ from the actual errors")
}
