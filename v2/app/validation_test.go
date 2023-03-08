package app

import (
	"bytes"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
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
)

func TestValidation(t *testing.T) {
	t.Parallel()
	testscript.Run(t, testscript.Params{
		Dir: "../../parser/testdata",
		Setup: func(env *testscript.Env) error {
			if err := testutil.TestScriptSetupFunc(env); err != nil {
				return err
			}
			env.Values["stderr"] = &bytes.Buffer{}
			env.Values["stdout"] = &bytes.Buffer{}
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
					printf("svc %s dbs=%s", svc.Name, "") // FIXME: bring databases in
				}

				for _, svc := range desc.Services {
					svc.Framework.ForAll(func(fw *apiframework.ServiceDesc) {

						for _, rpc := range fw.Endpoints {
							recvName := option.Map(rpc.Recv, func(recv *schema2.Receiver) string {
								switch t := recv.Type.(type) {
								case *schema2.NamedType:
									return "*" + t.Decl().Name
								case *schema2.PointerType:
									return "*" + t.Elem.(*schema2.NamedType).Decl().Name
								default:
									panic("a reciver should only be a named type or pointer type")
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
				for _, res := range desc.InfraResources {
					switch res := res.(type) {
					case *pubsub.Topic:
						res.BoundTo().ForAll(func(boundTo pkginfo.QualifiedName) {
							topicsByName[boundTo] = res
						})
					}
				}

				for _, res := range desc.InfraResources {
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
					case *pubsub.Subscription:
					// svc, found := desc.ServiceForPath(res.File.FSPath)
					// if !found {
					// 	ts.Fatalf("could not find service for path %s", res.File.FSPath)
					// }
					// TODO: implement this at the parser as we don't currently take the config
					// printf("pubsubSubscriber %s %s %s %d %d %d %d %d", topicsByName[res.Topic].Name, res.Name, svc.Name, res.AckDeadline, res.MessageRetention, res.MaxRetries, res.MinRetryBackoff, res.MaxRetryBackoff)
					case *metrics.Metric:
						// TODO: implement this at the parser as we don't currently take the metric kind or labels in the same way
						// printf("metric %s %s %s %s\n", res.Name, res.ValueType, res.Kind, res.Labels)
					}
				}

				// TODO: when we have infra usage tracking
				// for _, res := range desc.InfraUsages {
				// 	switch res := res.(type) {
				// 	case *pubsub.Publisher:
				// 		fmt.Fprintf(stdout, "pubsubTopic %s\n", topic.Name)
				//
				// 		for _, pub := range topic.Publishers {
				// 			if pub.Service != nil {
				// 				fmt.Fprintf(stdout, "pubsubPublisher %s %s\n", topic.Name, pub.Service.Name)
				// 			}
				// 			if pub.GlobalMiddleware != nil {
				// 				fmt.Fprintf(stdout, "pubsubPublisher middlware %s %s\n", topic.Name, pub.GlobalMiddleware.Name)
				// 			}
				// 		}					}
				// }
			},

			// The "output" command checks that the output into stdout that we've collected
			// contains the given regex
			"output": func(ts *testscript.TestScript, neg bool, args []string) {
				c := testutil.GetTestC(ts)

				matcher := qt.Contains
				if neg {
					matcher = qt.Not(matcher)
				}

				c.Assert(
					ts.Value("stdout").(*bytes.Buffer).String(), matcher, args[0],
				)
			},

			// The "err" command checks that the output into stderr that we've collected
			// contains the given regex
			"err": func(ts *testscript.TestScript, neg bool, args []string) {
				c := testutil.GetTestC(ts)

				matcher := qt.Contains
				if neg {
					matcher = qt.Not(matcher)
				}

				c.Assert(
					ts.Value("stderr").(*bytes.Buffer).String(), matcher, args[0],
				)
			},
		},
	})
}
