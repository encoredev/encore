package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/sqlc-dev/sqlc/pkg/cli"
	"google.golang.org/protobuf/encoding/protojson"

	"encr.dev/proto/encore/daemon"
)

type sqlcSQL struct {
	Schema  string        `json:"schema"`
	Queries string        `json:"queries"`
	Engine  string        `json:"engine"`
	Codegen []sqlcCodegen `json:"codegen"`
}

type sqlcCodegen struct {
	Out    string `json:"out"`
	Plugin string `json:"plugin"`
}

type sqlcPlugin struct {
	Name    string      `json:"name"`
	Process sqlcProcess `json:"process"`
}

type sqlcProcess struct {
	Cmd string `json:"cmd"`
}

type sqlcConfig struct {
	Version string       `json:"version"`
	SQL     []sqlcSQL    `json:"sql"`
	Plugins []sqlcPlugin `json:"plugins"`
}

func init() {
	var useProto bool
	genCmd := &cobra.Command{
		Use:    "generate-sql-schema <migration-dir>",
		Short:  "Plugin for SQLC: stores the parsed sqlc model in a protobuf file",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schemaPath, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			tmpDir, err := os.MkdirTemp("", "encore-sqlc")
			if err != nil {
				return err
			}
			defer func() {
				_ = os.RemoveAll(tmpDir)
			}()

			sqlcPath := filepath.Join(tmpDir, "sqlc.json")
			queryPath := filepath.Join(tmpDir, "query.sql")
			outPath := filepath.Join(tmpDir, "gen")
			// SQLC requires the schema path to be relative to the sqlc.json file
			schemaPath, err = filepath.Rel(tmpDir, schemaPath)
			if err != nil {
				return err
			}
			cfg := sqlcConfig{
				Version: "2",
				SQL: []sqlcSQL{
					{
						Schema:  schemaPath,
						Queries: "query.sql",
						Engine:  "postgresql",
						Codegen: []sqlcCodegen{
							{
								Out:    "gen",
								Plugin: "encore",
							},
						},
					},
				},
				Plugins: []sqlcPlugin{
					{
						Name: "encore",
						Process: sqlcProcess{
							Cmd: os.Args[0],
						},
					},
				},
			}
			cfgData, err := json.Marshal(cfg)
			if err != nil {
				return err
			}
			err = os.WriteFile(sqlcPath, cfgData, 0644)
			if err != nil {
				return err
			}

			// SQLC requires at least one query to be present in the query file
			err = os.WriteFile(queryPath, []byte("-- name: Dummy :one\nSELECT 'dummy';"), 0644)
			if err != nil {
				return err
			}

			res := cli.Run([]string{"generate", "-f", sqlcPath})
			if res != 0 {
				return fmt.Errorf("sqlc exited with code %d", res)
			}
			reqBlob, err := os.ReadFile(filepath.Join(outPath, "output.pb"))
			if !useProto {
				req := &daemon.SQLCPlugin_GenerateRequest{}
				if err := proto.Unmarshal(reqBlob, req); err != nil {
					return err
				}
				reqBlob, err = protojson.MarshalOptions{
					EmitUnpopulated: true,
					Indent:          "  ",
					UseProtoNames:   true,
				}.Marshal(req)
				if err != nil {
					return err
				}
			}

			w := bufio.NewWriter(os.Stdout)
			if _, err := w.Write(reqBlob); err != nil {
				return err
			}
			if err := w.Flush(); err != nil {
				return err
			}
			return nil
		},
	}
	genCmd.Flags().BoolVar(&useProto, "proto", false, "Output the parsed schema as protobuf")
	pluginCmd := &cobra.Command{
		Use:    "/plugin.CodegenService/Generate",
		Short:  "Plugin for SQLC: stores the parsed sqlc model in a protobuf file",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			reqBlob, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			resp := &daemon.SQLCPlugin_GenerateResponse{
				Files: []*daemon.SQLCPlugin_File{
					{
						Name:     "output.pb",
						Contents: reqBlob,
					},
				},
			}
			respBlob, err := proto.Marshal(resp)
			if err != nil {
				return err
			}
			w := bufio.NewWriter(os.Stdout)
			if _, err := w.Write(respBlob); err != nil {
				return err
			}
			if err := w.Flush(); err != nil {
				return err
			}
			return nil
		},
	}
	rootCmd.AddCommand(genCmd)
	rootCmd.AddCommand(pluginCmd)
}
