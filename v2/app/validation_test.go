package app

import (
	"bytes"
	"fmt"
	goregexp "regexp"
	"testing"

	"cuelang.org/go/pkg/regexp"
	"github.com/rogpeppe/go-internal/testscript"

	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/option"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	schema2 "encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/testutil"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/resource/config"
	"encr.dev/v2/parser/infra/resource/cron"
	"encr.dev/v2/parser/infra/resource/metrics"
	"encr.dev/v2/parser/infra/resource/pubsub"
	"encr.dev/v2/parser/infra/usage"
)

func TestValidation(t *testing.T) {
	type testCfg struct {
		ignoreErrCommand    bool
		ignoreOutputCommand bool
	}
	t.Parallel()
	testscript.Run(t, testscript.Params{
		Dir: "../../parser/testdata",
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
							recvName := option.Map(rpc.Recv, func(recv *schema2.Receiver) string {
								switch t := recv.Type.(type) {
								case *schema2.NamedType:
									return "*" + t.Decl().Name
								case schema2.NamedType:
									return "*" + t.Decl().Name
								case *schema2.PointerType:
									return "*" + t.Elem.(*schema2.NamedType).Decl().Name
								case schema2.PointerType:
									return "*" + t.Elem.(schema2.NamedType).Decl().Name
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
				for _, res := range desc.Infra.Resources() {
					switch res := res.(type) {
					case *pubsub.Topic:
						for _, b := range desc.Infra.PkgDeclBinds(res) {
							topicsByName[b.QualifiedName()] = res
						}
					}
				}

				for _, res := range desc.Infra.Resources() {
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

						for _, u := range desc.Infra.Usages(res) {
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

			// expectError is a command that checks the stderr output contains the
			// given regex.
			//
			// This function will also convert all whitespaces into a single space
			// this is to allow for tests to be written in a more readable way without
			// having to worry about the word wrapping of the error messages.
			//
			// It unique to the v2 parser as that has different error handling, as such
			// if it is used in a testscript, we ignore any following calls to "err"
			"expectError": func(ts *testscript.TestScript, neg bool, args []string) {
				ts.Value("cfg").(*testCfg).ignoreErrCommand = true

				stderr := ts.Value("stderr").(*bytes.Buffer)
				output := goregexp.MustCompile(`\s+`).ReplaceAllString(stderr.String(), " ")
				m, err := regexp.Match(args[0], output)
				if err != nil {
					ts.Fatalf("invalid pattern: %v", err)
				}
				if !m && !neg {
					ts.Fatalf("stderr does not match %q", args[0])
				} else if m && neg {
					ts.Fatalf("stderr unexpectedly matches %q", args[0])
				}
			},

			// The "err" command checks that the output into stderr that we've collected
			// contains the given regex
			"err": func(ts *testscript.TestScript, neg bool, args []string) {
				if ts.Value("cfg").(*testCfg).ignoreErrCommand {
					// "expectError" was called, so we ignore this command
					return
				}

				stderr := ts.Value("stderr").(*bytes.Buffer)
				m, err := regexp.Match(args[0], stderr.String())
				if err != nil {
					ts.Fatalf("invalid pattern: %v", err)
				}
				if !m && !neg {
					ts.Fatalf("stderr does not match %q", args[0])
				} else if m && neg {
					ts.Fatalf("stderr unexpectedly matches %q", args[0])
				}
			},
		},
	})
}
