package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"encr.dev/cli/daemon/sqldb/docker"
	daemonpb "encr.dev/proto/encore/daemon"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
}

var (
	resetAll bool
	testDB   bool
)

var dbResetCmd = &cobra.Command{
	Use:   "reset <database-names...|--all>",
	Short: "Resets the databases with the given names. Use --all to reset all databases.",

	Run: func(command *cobra.Command, args []string) {
		appRoot, _ := determineAppRoot()
		dbNames := args
		if resetAll {
			if len(dbNames) > 0 {
				fatal("cannot specify both --all and database names")
			}
			dbNames = nil
		} else {
			if len(dbNames) == 0 {
				fatal("no database names given")
			}
		}

		ctx := context.Background()
		daemon := setupDaemon(ctx)
		stream, err := daemon.DBReset(ctx, &daemonpb.DBResetRequest{
			AppRoot:       appRoot,
			DatabaseNames: dbNames,
			Test:          testDB,
		})
		if err != nil {
			fatal("reset databases: ", err)
		}
		os.Exit(streamCommandOutput(stream, nil))
	},
}

var dbEnv string

var dbShellCmd = &cobra.Command{
	Use:   "shell DATABASE_NAME [--test] [--env=<name>]",
	Short: "Connects to the database via psql shell",
	Long: `Defaults to connecting to your local environment.
Specify --env to connect to another environment.

Use --test to connect to databases used for integration testing.
--test implies --env=local.
`,
	Args: cobra.MaximumNArgs(1),

	DisableFlagsInUseLine: true,
	Run: func(command *cobra.Command, args []string) {
		appRoot, relPath := determineAppRoot()
		ctx := context.Background()
		daemon := setupDaemon(ctx)
		dbName := ""
		if len(args) > 0 {
			dbName = args[0]
			// Ignore the trailing slash to support auto-completion of directory names
			dbName = strings.TrimSuffix(dbName, "/")
		} else {
			// Find the enclosing service by looking for the "migrations" folder
		SvcNameLoop:
			for p := relPath; p != "."; p = filepath.Dir(p) {
				absPath := filepath.Join(appRoot, p)
				if _, err := os.Stat(filepath.Join(absPath, "migrations")); err == nil {
					pkgs, err := resolvePackages(absPath, ".")
					if err == nil && len(pkgs) > 0 {
						dbName = filepath.Base(pkgs[0])
						break SvcNameLoop
					}
				}
			}
			if dbName == "" {
				fatal("could not find an Encore service with a database in this directory (or any of the parent directories).\n\n" +
					"Note: You can specify a service name to connect to it directly using the command 'encore db shell <database-name>'.")
			}
		}

		if testDB {
			dbEnv = "local"
		}

		resp, err := daemon.DBConnect(ctx, &daemonpb.DBConnectRequest{
			AppRoot: appRoot,
			DbName:  dbName,
			EnvName: dbEnv,
			Test:    testDB,
		})
		if err != nil {
			fatalf("could not connect to the database for service %s: %v", dbName, err)
		}

		// If we have the psql binary, use that.
		// Otherwise fall back to docker.
		var cmd *exec.Cmd
		if p, err := exec.LookPath("psql"); err == nil {
			cmd = exec.Command(p, resp.Dsn)
		} else {
			fmt.Fprintln(os.Stderr, "encore: no 'psql' executable found in $PATH; using docker to run 'psql' instead.\n\nNote: install psql to hide this message.")
			dsn := resp.Dsn

			if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
				// Docker for {Mac, Windows}'s networking setup requires
				// using "host.docker.internal" instead of "localhost"
				for _, rep := range []string{"localhost", "127.0.0.1"} {
					dsn = strings.Replace(dsn, rep, "host.docker.internal", -1)
				}
			}

			cmd = exec.Command("docker", "run", "-it", "--rm", "--network=host", docker.Image, "psql", dsn)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Start(); err != nil {
			log.Fatal().Err(err).Msg("failed to start psql")
		}
		signal.Ignore(os.Interrupt)
		if err := cmd.Wait(); err != nil {
			log.Fatal().Err(err).Msg("psql failed")
		}
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveFilterDirs
	},
}

var dbProxyPort int32

var dbProxyCmd = &cobra.Command{
	Use:   "proxy [--env=<name>] [--test]",
	Short: "Sets up a proxy tunnel to the database",
	Long: `Set up a proxy tunnel to a database for use with other tools.

Use --test to connect to databases used for integration testing.
--test implies --env=local.
`,

	Run: func(command *cobra.Command, args []string) {
		appRoot, _ := determineAppRoot()
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-interrupt
			cancel()
		}()

		if testDB {
			dbEnv = "local"
		}

		daemon := setupDaemon(ctx)
		stream, err := daemon.DBProxy(ctx, &daemonpb.DBProxyRequest{
			AppRoot: appRoot,
			EnvName: dbEnv,
			Port:    dbProxyPort,
			Test:    testDB,
		})
		if err != nil {
			log.Fatal().Err(err).Msg("could not setup db proxy")
		}
		os.Exit(streamCommandOutput(stream, nil))
	},
}

var dbConnURICmd = &cobra.Command{
	Use:   "conn-uri [<db-name>] [--test]",
	Short: "Outputs the database connection string",
	Args:  cobra.MaximumNArgs(1),

	Run: func(command *cobra.Command, args []string) {
		appRoot, relPath := determineAppRoot()
		ctx := context.Background()
		daemon := setupDaemon(ctx)
		dbName := ""
		if len(args) > 0 {
			dbName = args[0]
		} else {
			// Find the enclosing service by looking for the "migrations" folder
		DBNameLoop:
			for p := relPath; p != "."; p = filepath.Dir(p) {
				absPath := filepath.Join(appRoot, p)
				if _, err := os.Stat(filepath.Join(absPath, "migrations")); err == nil {
					pkgs, err := resolvePackages(absPath, ".")
					if err == nil && len(pkgs) > 0 {
						dbName = filepath.Base(pkgs[0])
						break DBNameLoop
					}
				}
			}
			if dbName == "" {
				fatal("could not find Encore service with a database in this directory (or any parent directory).\n\n" +
					"Note: You can specify a service name to connect to it directly using the command 'encore db conn-uri <service-name>'.")
			}
		}

		if testDB {
			dbEnv = "local"
		}

		resp, err := daemon.DBConnect(ctx, &daemonpb.DBConnectRequest{
			AppRoot: appRoot,
			DbName:  dbName,
			EnvName: dbEnv,
			Test:    testDB,
		})
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				if st.Code() == codes.NotFound {
					fatalf("no such database found: %s", dbName)
				}
			}
			fatalf("could not connect to the database for service %s: %v", dbName, err)
		}

		fmt.Fprintln(os.Stdout, resp.Dsn)
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)

	dbResetCmd.Flags().BoolVar(&resetAll, "all", false, "Reset all services in the application")
	dbCmd.AddCommand(dbResetCmd)

	dbShellCmd.Flags().StringVarP(&dbEnv, "env", "e", "local", "Environment name to connect to (such as \"prod\")")
	dbShellCmd.Flags().BoolVarP(&testDB, "test", "t", false, "Connect to the integration test database (implies --env=local)")
	dbCmd.AddCommand(dbShellCmd)

	dbProxyCmd.Flags().StringVarP(&dbEnv, "env", "e", "local", "Environment name to connect to (such as \"prod\")")
	dbProxyCmd.Flags().Int32VarP(&dbProxyPort, "port", "p", 0, "Port to listen on (defaults to a random port)")
	dbProxyCmd.Flags().BoolVarP(&testDB, "test", "t", false, "Connect to the integration test database (implies --env=local)")
	dbCmd.AddCommand(dbProxyCmd)

	dbConnURICmd.Flags().StringVarP(&dbEnv, "env", "e", "local", "Environment name to connect to (such as \"prod\")")
	dbConnURICmd.Flags().BoolVarP(&testDB, "test", "t", false, "Connect to the integration test database (implies --env=local)")
	dbCmd.AddCommand(dbConnURICmd)
}
