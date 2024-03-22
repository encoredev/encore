package ai

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/golang/protobuf/proto"

	"encr.dev/cli/daemon/apps"
	"encr.dev/proto/encore/daemon"
)

func ParseSQLSchema(app *apps.Instance, schema string) error {
	schemaPath := filepath.Join(app.Root(), schema)
	cmd := exec.Command(os.Args[0], "generate-sql-schema", "--proto", schemaPath)
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	var req daemon.SQLCPlugin_GenerateRequest
	if err := proto.Unmarshal(output, &req); err != nil {
		return err
	}
	return nil
}
