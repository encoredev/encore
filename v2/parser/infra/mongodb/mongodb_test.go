package mongodb

import (
	"testing"

	"encr.dev/v2/parser/resource/resourcetest"
)

func TestParseDatabase(t *testing.T) {
	tests := []resourcetest.Case[*Database]{
		{
			Name: "basic",
			Code: `
// Database docs
var x = mongodb.NewDatabase("name", mongodb.DatabaseConfig{})
`,
			Want: &Database{
				Name: "name",
				Doc:  "Database docs\n",
			},
		},
	}

	resourcetest.Run(t, DatabaseParser, tests)
}
