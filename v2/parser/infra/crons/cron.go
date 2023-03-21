package crons

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"

	cronparser "github.com/robfig/cron/v3"

	"encr.dev/pkg/errors"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	literals "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	parseutil "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

type Job struct {
	AST      *ast.CallExpr
	File     *pkginfo.File
	Name     string // The unique name of the cron job
	Doc      string // The documentation on the cron job
	Title    string // cron job title
	Schedule string

	Endpoint    pkginfo.QualifiedName // The Endpoint reference
	EndpointAST ast.Node
}

func (j *Job) Kind() resource.Kind       { return resource.CronJob }
func (j *Job) Package() *pkginfo.Package { return j.File.Pkg }
func (j *Job) ASTExpr() ast.Expr         { return j.AST }
func (j *Job) ResourceName() string      { return j.Name }
func (j *Job) Pos() token.Pos            { return j.AST.Pos() }
func (j *Job) End() token.Pos            { return j.AST.End() }

var JobParser = &resourceparser.Parser{
	Name: "Cron Job",

	InterestingImports: []paths.Pkg{"encore.dev/cron"},
	Run: func(p *resourceparser.Pass) {
		name := pkginfo.QualifiedName{PkgPath: "encore.dev/cron", Name: "NewJob"}

		spec := &parseutil.ReferenceSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 0,
			MaxTypeArgs: 0,
			Parse:       parseCronJob,
		}

		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			parseutil.ParseReference(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

const (
	minute int64 = 60
	hour         = 60 * minute
)

var cronjobParser = cronparser.NewParser(cronparser.Minute | cronparser.Hour | cronparser.Dom | cronparser.Month | cronparser.Dow)

func parseCronJob(d parseutil.ReferenceInfo) {
	displayName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 2 {
		d.Pass.Errs.Add(errExpects2Arguments(len(d.Call.Args)).AtGoNode(d.Call))
		return
	}

	jobName := parseutil.ParseResourceName(d.Pass.Errs, displayName, "cron job name",
		d.Call.Args[0], parseutil.KebabName, "")
	if jobName == "" {
		// we already reported the error inside ParseResourceName
		return
	}

	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "cron.JobConfig", d.Call.Args[1])
	if !ok {
		return // error reported by ParseStruct
	}

	// Decode the config
	type decodedConfig struct {
		Title    string   `literal:",optional"`
		Endpoint ast.Expr `literal:",required,dynamic"`
		Every    int64    `literal:",optional"`
		Schedule string   `literal:",optional"`
	}
	config := literals.Decode[decodedConfig](d.Pass.Errs, cfgLit, nil)

	// Resolve the endpoint
	endpoint, ok := d.File.Names().ResolvePkgLevelRef(config.Endpoint)
	if !ok {
		d.Pass.Errs.Add(
			errUnableToResolveEndpoint.AtGoNode(config.Endpoint),
		)
		return
	}

	job := &Job{
		AST:         d.Call,
		File:        d.File,
		Name:        jobName,
		Doc:         d.Doc,
		Title:       config.Title,
		Endpoint:    endpoint,
		EndpointAST: config.Endpoint,
	}
	if job.Title == "" {
		job.Title = jobName
	}

	// Parse the schedule
	switch {
	case config.Every != 0 && config.Schedule != "":
		d.Pass.Errs.Add(errScheduleSetTwice.AtGoNode(cfgLit.Expr("Schedule")).AtGoNode(cfgLit.Expr("Every")))
		return
	case config.Schedule != "":
		_, err := cronjobParser.Parse(config.Schedule)
		if err != nil {
			d.Pass.Errs.Add(errInvalidSchedule.Wrapping(err).AtGoNode(cfgLit.Expr("Schedule")))
			return
		}
		job.Schedule = fmt.Sprintf("schedule:%s", config.Schedule)
	case config.Every != 0:
		if rem := config.Every % minute; rem != 0 {
			d.Pass.Errs.Add(errEveryMustBeInteger(config.Every).AtGoNode(cfgLit.Expr("Every")))
			return
		}

		minutes := config.Every / minute
		if minutes < 1 {
			d.Pass.Errs.Add(errEveryMustBeOneOrGreater(config.Every).AtGoNode(cfgLit.Expr("Every")))
			return
		} else if minutes > 24*60 {
			d.Pass.Errs.Add(errEveryMustBeLessThan24Hours(minutes).AtGoNode(cfgLit.Expr("Every")))
			return
		} else if suggestion, ok := isCronIntervalAllowed(int(minutes)); !ok {
			suggestionStr := formatMinutes(suggestion)
			minutesStr := formatMinutes(int(minutes))

			d.Pass.Errs.Add(
				errEveryMustBeMultipleOfMinute(minutesStr).
					AtGoNode(cfgLit.Expr("Every"), errors.AsHelp(fmt.Sprintf("try setting it to %s", suggestionStr))),
			)
			return
		}
		job.Schedule = fmt.Sprintf("every:%d", minutes)
	}

	d.Pass.RegisterResource(job)
	if id, ok := d.Ident.Get(); ok {
		d.Pass.AddBind(id, job)
	}
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func formatMinutes(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d * cron.Minute", minutes)
	} else if minutes%60 == 0 {
		return fmt.Sprintf("%d * cron.Hour", minutes/60)
	}
	return fmt.Sprintf("%d * cron.Hour + %d * cron.Minute", minutes/60, minutes%60)
}

func isCronIntervalAllowed(val int) (suggestion int, ok bool) {
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
