package cron

import (
	"testing"

	"encr.dev/v2/parser/infra/resource/resourcetest"
)

func TestParseJob(t *testing.T) {
	tests := []resourcetest.Case[*Job]{
		{
			Name: "basic",
			Code: `
// Job docs
var x = cron.NewJob("name", cron.JobConfig{
	Title: "title",
	Every: 3 * cron.Hour,
	Endpoint: MyEndpoint,
})

func MyEndpoint() {}
`,
			Want: &Job{
				Name:     "name",
				Title:    "title",
				Doc:      "Job docs\n",
				Schedule: "every:180",
			},
		},
	}

	resourcetest.Run(t, JobParser, tests)
}
