package app

import (
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/cron"
)

func (d *Desc) validateCrons(pc *parsectx.Context, result *parser.Result) {
	foundCronjobs := make(map[string]*cron.Job)

	cronjobs := parser.Resources[*cron.Job](result)
	for _, cronjob := range cronjobs {
		if previous, ok := foundCronjobs[cronjob.Name]; ok {
			pc.Errs.Add(
				cron.ErrDuplicateNames.
					AtGoNode(cronjob.AST.Args[0]).
					AtGoNode(previous.AST.Args[0]),
			)
		}
		foundCronjobs[cronjob.Name] = cronjob
	}
}
