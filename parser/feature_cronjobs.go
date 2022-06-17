package parser

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"math/big"
	"sort"
	"strconv"

	cronparser "github.com/robfig/cron/v3"

	"encr.dev/parser/dnsname"
	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/names"
	"encr.dev/parser/internal/walker"
)

func init() {
	registerResource(
		est.CronJobResource,
		"cron job",
		"https://encore.dev/docs/develop/cron-jobs",
		"cron",
		cronImportPath,
	)

	registerResourceCreationParser(
		est.CronJobResource,
		"NewJob", 0,
		(*parser).parseCronJob,
		locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
	)
}

const (
	minute int64 = 60
	hour         = 60 * minute
)

var cronjobParser = cronparser.NewParser(cronparser.Minute | cronparser.Hour | cronparser.Dom | cronparser.Month | cronparser.Dow)

func (p *parser) parseCronJob(file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
	if len(callExpr.Args) != 2 {
		p.errf(callExpr.Pos(), "cron.NewJob must be called as (id string, cfg cron.JobConfig)")
		return nil
	}

	cj := &est.CronJob{
		AST:      ident,
		DeclFile: file,
	}

	cronJobID, _ := litString(callExpr.Args[0])
	if cronJobID == "" {
		p.errf(callExpr.Args[0].Pos(), "cron.NewJob must be called with a string literal as its first argument")
		return nil
	}
	err := dnsname.DNS1035Label(cronJobID)
	if err != nil {
		p.errf(callExpr.Pos(), "cron.NewJob: id must consist of lower case alphanumeric characters"+
			" or '-',\n// start with an alphabetic character, and end with an alphanumeric character ")
		return nil
	}
	cj.ID = cronJobID
	cj.Title = cronJobID // Set ID as the default title

	if cl, ok := callExpr.Args[1].(*ast.CompositeLit); ok {
		info := p.names[file.Pkg].Files[file]

		if imp, obj := pkgObj(info, cl.Type); imp == cronImportPath && obj == "JobConfig" {
			hasSchedule := false
			for _, e := range cl.Elts {
				kv := e.(*ast.KeyValueExpr)
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					p.errf(kv.Pos(), "field must be an identifier")
					return nil
				}
				switch key.Name {
				case "Title":
					if v, ok := kv.Value.(*ast.BasicLit); ok && v.Kind == token.STRING {
						parsed, _ := strconv.Unquote(v.Value)
						cj.Title = parsed
					} else {
						p.errf(v.Pos(), "Title must be a string literal")
						return nil
					}
				case "Every":
					if hasSchedule {
						p.errf(kv.Pos(), "Every: cron execution schedule was already defined using the Schedule field, at least one must be set but not both")
						return nil
					}
					if dur, ok := p.parseCronLiteral(info, kv.Value); ok {
						// We only support intervals that are a positive integer number of minutes.
						if rem := dur % minute; rem != 0 {
							p.errf(kv.Value.Pos(), "Every: must be an integer number of minutes, got %d", dur)
							return nil
						}

						minutes := dur / minute
						if minutes < 1 {
							p.errf(kv.Value.Pos(), "Every: duration must be one minute or greater, got %d", minutes)
							return nil
						} else if minutes > 24*60 {
							p.errf(kv.Value.Pos(), "Every: duration must not be greater than 24 hours (1440 minutes), got %d", minutes)
							return nil
						} else if suggestion, ok := p.isCronIntervalAllowed(int(minutes)); !ok {
							suggestionStr := p.formatMinutes(suggestion)
							minutesStr := p.formatMinutes(int(minutes))
							p.errf(kv.Value.Pos(), "Every: 24 hour time range (from 00:00 to 23:59) "+
								"needs to be evenly divided by the interval value (%s), try setting it to (%s)", minutesStr, suggestionStr)
							return nil
						}
						cj.Schedule = fmt.Sprintf("every:%d", minutes)
						hasSchedule = true
					} else {
						return nil
					}
				case "Schedule":
					if hasSchedule {
						p.errf(kv.Pos(), "cron execution schedule was already defined using the Every field, at least one must be set but not both")
						return nil
					}
					if v, ok := kv.Value.(*ast.BasicLit); ok && v.Kind == token.STRING {
						parsed, _ := strconv.Unquote(v.Value)
						_, err := cronjobParser.Parse(parsed)
						if err != nil {
							p.errf(v.Pos(), "Schedule must be a valid cron expression: %s", err)
							return nil
						}
						cj.Schedule = fmt.Sprintf("schedule:%s", parsed)
						hasSchedule = true
					} else {
						p.errf(v.Pos(), "Schedule must be a string literal")
						return nil
					}
				case "Endpoint":
					// This is one of the places where it's fine to reference an RPC endpoint.
					p.validRPCReferences[kv.Value] = true

					pkgPath, objName, _ := p.names.PackageLevelRef(file, kv.Value)
					if pkgPath != "" {
						if svc, found := p.svcPkgPaths[pkgPath]; found {
							for _, rpc := range svc.RPCs {
								if rpc.Func.Name.Name == objName {
									cj.RPC = rpc
									break
								}
							}
						}
					}

					if cj.RPC == nil {
						p.errf(kv.Value.Pos(), "Endpoint does not reference an Encore API")
						return nil
					}
				default:
					p.errf(key.Pos(), "cron.JobConfig has unknown key %s", key.Name)
					return nil
				}
			}

			if _, err := cj.IsValid(); err != nil {
				p.errf(cl.Pos(), "cron.NewJob: %s", err)
			}

			cj.Doc = cursor.DocComment()
			if cronJob2 := p.jobsMap[cj.ID]; cronJob2 != nil {
				p.errf(callExpr.Pos(), "cron job %s defined twice", cj.ID)
				return nil
			}

			p.jobs = append(p.jobs, cj)
			p.jobsMap[cj.ID] = cj

			return cj
		}
	}

	return nil
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (p *parser) formatMinutes(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d * cron.Minute", minutes)
	} else if minutes%60 == 0 {
		return fmt.Sprintf("%d * cron.Hour", minutes/60)
	}
	return fmt.Sprintf("%d * cron.Hour + %d * cron.Minute", minutes/60, minutes%60)
}

func (p *parser) isCronIntervalAllowed(val int) (suggestion int, ok bool) {
	allowed := []int{
		1, 2, 3, 4, 5, 6, 8, 9, 10, 12, 15, 16, 18, 20, 24, 30, 32, 36, 40, 45,
		48, 60, 72, 80, 90, 96, 120, 144, 160, 180, 240, 288, 360, 480, 720, 1440,
	}
	idx := sort.SearchInts(allowed, val)

	if idx == len(allowed) {
		return allowed[len(allowed)-1], false
	} else if allowed[idx] == val {
		return val, true
	} else if idx == 0 {
		return allowed[0], false
	} else if abs(val-allowed[idx-1]) < abs(val-allowed[idx]) {
		return allowed[idx-1], false
	}

	return allowed[idx], false
}

// parseCronLiteral parses an expression representing a cron duration constant.
// It uses go/constant to perform arbitrary-precision arithmetic according
// to the rules of the Go compiler.
func (p *parser) parseCronLiteral(info *names.File, durationExpr ast.Expr) (dur int64, ok bool) {
	zero := constant.MakeInt64(0)
	var parse func(expr ast.Expr) constant.Value
	parse = func(expr ast.Expr) constant.Value {
		switch x := expr.(type) {
		case *ast.BinaryExpr:
			lhs := parse(x.X)
			rhs := parse(x.Y)
			switch x.Op {
			case token.MUL, token.ADD, token.SUB, token.REM, token.AND, token.OR, token.XOR, token.AND_NOT:
				return constant.BinaryOp(lhs, x.Op, rhs)
			case token.QUO:
				// constant.BinaryOp panics when dividing by zero
				if constant.Compare(rhs, token.EQL, zero) {
					p.errf(x.Pos(), "cannot divide by zero")
					return constant.MakeUnknown()
				}

				return constant.BinaryOp(lhs, x.Op, rhs)
			default:
				p.errf(x.Pos(), "unsupported operation: %s", x.Op)
				return constant.MakeUnknown()
			}

		case *ast.UnaryExpr:
			val := parse(x.X)
			switch x.Op {
			case token.ADD, token.SUB, token.XOR:
				return constant.UnaryOp(x.Op, val, 0)
			default:
				p.errf(x.Pos(), "unsupported operation: %s", x.Op)
				return constant.MakeUnknown()
			}

		case *ast.BasicLit:
			switch x.Kind {
			case token.INT, token.FLOAT:
				return constant.MakeFromLiteral(x.Value, x.Kind, 0)
			default:
				p.errf(x.Pos(), "unsupported literal in duration expression: %s", x.Kind)
				return constant.MakeUnknown()
			}

		case *ast.CallExpr:
			// We allow "cron.Duration(x)" as a no-op
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Duration" {
				if id, ok := sel.X.(*ast.Ident); ok {
					ri := info.Idents[id]
					if ri != nil && ri.ImportPath == cronImportPath {
						if len(x.Args) == 1 {
							return parse(x.Args[0])
						}
					}
				}
			}
			p.errf(x.Pos(), "unsupported call expression in duration expression")
			return constant.MakeUnknown()

		case *ast.SelectorExpr:
			if pkg, obj := pkgObj(info, x); pkg == cronImportPath {
				var d int64
				switch obj {
				case "Minute":
					d = minute
				case "Hour":
					d = hour
				default:
					p.errf(x.Pos(), "unsupported duration value: %s.%s (expected cron.Minute or cron.Hour)", pkg, obj)
					return constant.MakeUnknown()
				}
				return constant.MakeInt64(d)
			}
			p.errf(x.Pos(), "unexpected value in duration literal")
			return constant.MakeUnknown()

		case *ast.ParenExpr:
			return parse(x.X)

		default:
			p.errf(x.Pos(), "unsupported expression in duration literal: %T", x)
			return constant.MakeUnknown()
		}
	}

	val := constant.Val(parse(durationExpr))
	switch val := val.(type) {
	case int64:
		return val, true
	case *big.Int:
		if !val.IsInt64() {
			p.errf(durationExpr.Pos(), "duration expression out of bounds")
			return 0, false
		}
		return val.Int64(), true
	case *big.Rat:
		num := val.Num()
		if val.IsInt() && num.IsInt64() {
			return num.Int64(), true
		}
		p.errf(durationExpr.Pos(), "floating point numbers are not supported in duration literals")
		return 0, false
	case *big.Float:
		p.errf(durationExpr.Pos(), "floating point numbers are not supported in duration literals")
		return 0, false
	default:
		p.errf(durationExpr.Pos(), "unsupported duration literal")
		return 0, false
	}
}
