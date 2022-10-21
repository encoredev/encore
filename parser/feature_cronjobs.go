package parser

import (
	"fmt"
	"go/ast"
	"sort"

	cronparser "github.com/robfig/cron/v3"

	"encr.dev/internal/experiment"
	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
)

func init() {
	registerResource(est.CronJobResource, "cron job", "https://encore.dev/docs/develop/cron-jobs", "cron", cronImportPath, experiment.None)

	registerResourceCreationParser(est.CronJobResource, "NewJob", 0, (*parser).parseCronJob, experiment.None, locations.AllowedIn(locations.Variable).ButNotIn(locations.Function))
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

	cronJobID := p.parseResourceName("cron.NewJob", "cronjob ID", callExpr.Args[0])
	if cronJobID == "" {
		// error already reported
		return nil
	}

	// Parse the literal struct representing the job configuration
	cfg, ok := p.parseStructLit(file, "cron.JobConfig", callExpr.Args[1])
	if !ok {
		return nil
	}
	// Check everything apart from Handler is constant
	ok = true
	for fieldName, expr := range cfg.DynamicFields() {
		if fieldName != "Endpoint" {
			p.errf(expr.Pos(), "All values in cron.JobConfig must be a constant, however %s was not a constant, got %s", fieldName, prettyPrint(expr))
			ok = false
		}
	}
	if !ok {
		return nil
	}

	cj.ID = cronJobID
	cj.Title = cfg.Str("Title", cronJobID)

	// Parse the schedule
	switch {
	case cfg.IsSet("Every") && cfg.IsSet("Schedule"):
		p.errf(cfg.Pos("Every"), "Cron execution schedule was set twice, once in Every and one in Schedule, at least one must be set but not both")
		return nil
	case cfg.IsSet("Schedule"):
		parsed := cfg.Str("Schedule", "")
		_, err := cronjobParser.Parse(parsed)
		if err != nil {
			p.errf(cfg.Pos("Schedule"), "Schedule must be a valid cron expression: %s", err)
			return nil
		}
		cj.Schedule = fmt.Sprintf("schedule:%s", parsed)
	case cfg.IsSet("Every"):
		dur := cfg.Int64("Every", 0)
		if rem := dur % minute; rem != 0 {
			p.errf(cfg.Pos("Every"), "Every: must be an integer number of minutes, got %d", dur)
			return nil
		}

		minutes := dur / minute
		if minutes < 1 {
			p.errf(cfg.Pos("Every"), "Every: duration must be one minute or greater, got %d", minutes)
			return nil
		} else if minutes > 24*60 {
			p.errf(cfg.Pos("Every"), "Every: duration must not be greater than 24 hours (1440 minutes), got %d", minutes)
			return nil
		} else if suggestion, ok := p.isCronIntervalAllowed(int(minutes)); !ok {
			suggestionStr := p.formatMinutes(suggestion)
			minutesStr := p.formatMinutes(int(minutes))
			p.errf(cfg.Pos("Every"), "Every: 24 hour time range (from 00:00 to 23:59) "+
				"needs to be evenly divided by the interval value (%s), try setting it to (%s)", minutesStr, suggestionStr)
			return nil
		}
		cj.Schedule = fmt.Sprintf("every:%d", minutes)
	}

	// Parse the endpoint
	{
		endpoint := cfg.Expr("Endpoint")
		if endpoint == nil {
			p.errf(cfg.Pos("Endpoint"), "Endpoint must be defined in cron.JobConfig.")
			return nil
		}

		// This is one of the places where it's fine to reference an RPC endpoint.
		p.validRPCReferences[endpoint] = true
		rpc, ok := p.resolveRPCRef(file, endpoint)
		if !ok {
			p.errf(endpoint.Pos(), "Endpoint does not reference an Encore API")
			return nil
		}
		cj.RPC = rpc
	}

	if _, err := cj.IsValid(); err != nil {
		p.errf(callExpr.Pos(), "cron.NewJob: %s", err)
		return nil
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
