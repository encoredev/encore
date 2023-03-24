package app

import (
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/crons"
	"encr.dev/v2/parser/resource"
)

func (d *Desc) validateCrons(pc *parsectx.Context, result *parser.Result) {
	foundCronjobs := make(map[string]*crons.Job)

	cronjobs := parser.Resources[*crons.Job](result)
	for _, cronjob := range cronjobs {
		if previous, ok := foundCronjobs[cronjob.Name]; ok {
			pc.Errs.Add(
				crons.ErrDuplicateNames.
					AtGoNode(cronjob.AST.Args[0]).
					AtGoNode(previous.AST.Args[0]),
			)
		}
		foundCronjobs[cronjob.Name] = cronjob

		res, ok := result.ResourceForQN(cronjob.Endpoint).Get()
		if !ok || res.Kind() != resource.APIEndpoint {
			pc.Errs.Add(
				crons.ErrEndpointNotAnAPI.AtGoNode(cronjob.EndpointAST),
			)
			continue
		}
	}
}
