package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/platform"
	"encr.dev/pkg/appfile"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/pgproxy"
	daemonpb "encr.dev/proto/encore/daemon"
)

func toRoleType(role daemonpb.DBRole) sqldb.RoleType {
	switch role {
	case daemonpb.DBRole_DB_ROLE_READ:
		return sqldb.RoleRead
	case daemonpb.DBRole_DB_ROLE_WRITE:
		return sqldb.RoleWrite
	case daemonpb.DBRole_DB_ROLE_ADMIN:
		return sqldb.RoleAdmin
	case daemonpb.DBRole_DB_ROLE_SUPERUSER:
		return sqldb.RoleSuperuser
	default:
		return sqldb.RoleRead
	}

}

// DBConnect starts the database and returns the DSN for connecting to it.
func (s *Server) DBConnect(ctx context.Context, req *daemonpb.DBConnectRequest) (*daemonpb.DBConnectResponse, error) {
	if req.EnvName == "local" {
		return s.dbConnectLocal(ctx, req)
	}

	appID, err := appfile.Slug(req.AppRoot)
	if err != nil {
		return nil, err
	} else if appID == "" {
		return nil, errNotLinked
	}
	port, passwd, err := sqldb.OneshotProxy(appID, req.EnvName, toRoleType(req.Role))
	if err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("postgresql://encore:%s@127.0.0.1:%d/%s?sslmode=disable", passwd, port, req.DbName)
	return &daemonpb.DBConnectResponse{Dsn: dsn}, nil
}

func (s *Server) dbConnectLocal(ctx context.Context, req *daemonpb.DBConnectRequest) (*daemonpb.DBConnectResponse, error) {
	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		return nil, err
	}

	expSet, err := app.Experiments(nil)
	if err != nil {
		return nil, err
	}

	// Parse the app to figure out what infrastructure is needed.
	bld := builderimpl.Resolve(app.Lang(), expSet)
	defer fns.CloseIgnore(bld)
	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       builder.DefaultBuildInfo(),
		App:         app,
		Experiments: expSet,
		WorkingDir:  ".",
		ParseTests:  false,
	})
	if err != nil {
		return nil, err
	}

	// The Encore IDE plugins will request a connection to the database "_any_"
	// as they will be unaware of any database names ahead of time.
	//
	// We will use the first database name in the app's schema on the returned connection string
	if req.DbName == "_any_" {
		req.DbName = ""
		if len(parse.Meta.SqlDatabases) > 0 {
			req.DbName = parse.Meta.SqlDatabases[0].Name
		}

		// If no database has been found, return an error
		if req.DbName == "" {
			return nil, errDatabaseNotFound
		}
	} else {
		// Otherwise we need to check the requested service exists
		databaseExists := false
		for _, s := range parse.Meta.SqlDatabases {
			if s.Name == req.DbName {
				databaseExists = true
				break
			}
		}
		if !databaseExists {
			return nil, errDatabaseNotFound
		}
	}

	clusterNS, err := s.namespaceOrActive(ctx, app, req.Namespace)
	if err != nil {
		return nil, err
	}

	var passwd string
	clusterType := getClusterType(req)
	switch clusterType {
	case sqldb.Run:
		// If the user didn't specify a namespace, leave it out from the password
		// so it uses the active namespace.
		if req.Namespace != nil {
			passwd = "local-" + string(clusterNS.ID)
		} else {
			passwd = "local"
		}
	default:
		passwd = fmt.Sprintf("%s-%s", clusterType, clusterNS.ID)
	}

	clusterID := sqldb.GetClusterID(app, clusterType, clusterNS)
	log := log.With().Interface("cluster", clusterID).Logger()
	log.Info().Msg("setting up database cluster")
	cluster := s.cm.Create(ctx, &sqldb.CreateParams{
		ClusterID: clusterID,
		Memfs:     clusterType.Memfs(),
	})
	if cluster.IsExternalDB(req.DbName) {
		return nil, errors.New("connecting to an external database is disabled")
	}
	// TODO would be nice to stream this to the CLI
	if _, err := cluster.Start(ctx, nil); err != nil {
		log.Error().Err(err).Msg("failed to start db cluster")
		return nil, err
	} else if err := cluster.Setup(ctx, req.AppRoot, parse.Meta); err != nil {
		log.Error().Err(err).Msg("failed to create databases")
		return nil, err
	}
	log.Info().Msg("created database cluster")

	dsn := fmt.Sprintf("postgresql://%s:%s@127.0.0.1:%d/%s?sslmode=disable",
		app.PlatformOrLocalID(), passwd, s.mgr.DBProxyPort, req.DbName)
	return &daemonpb.DBConnectResponse{Dsn: dsn}, nil
}

// DBProxy starts a local database proxy for connecting to remote databases
// on the encore.dev platform.
func (s *Server) DBProxy(params *daemonpb.DBProxyRequest, stream daemonpb.Daemon_DBProxyServer) (err error) {
	ctx := stream.Context()

	appID, err := appfile.Slug(params.AppRoot)
	if err != nil {
		return err
	} else if appID == "" && params.EnvName != "local" {
		return errNotLinked
	}

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:"+strconv.Itoa(int(params.Port)))
	if err != nil {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	log.Info().Msgf("dbproxy: listening on localhost:%d", port)
	defer log.Info().Msg("dbproxy: proxy closed")
	err = stream.Send(&daemonpb.CommandMessage{Msg: &daemonpb.CommandMessage_Output{
		Output: &daemonpb.CommandOutput{
			Stdout: []byte(fmt.Sprintf("dbproxy: listening for TCP connections on localhost:%d\n", port)),
		},
	}})
	if err != nil {
		return err
	}

	var runProxy func() error
	if params.EnvName == "local" {
		app, err := s.apps.Track(params.AppRoot)
		if err != nil {
			return err
		}

		expSet, err := app.Experiments(nil)
		if err != nil {
			return err
		}

		// Parse the app to figure out what infrastructure is needed.
		bld := builderimpl.Resolve(app.Lang(), expSet)
		defer fns.CloseIgnore(bld)
		parse, err := bld.Parse(ctx, builder.ParseParams{
			Build:       builder.DefaultBuildInfo(),
			App:         app,
			Experiments: expSet,
			WorkingDir:  ".",
			ParseTests:  false,
		})
		if err != nil {
			return err
		}

		clusterType := getClusterType(params)

		clusterNS, err := s.namespaceOrActive(stream.Context(), app, params.Namespace)
		if err != nil {
			return err
		}

		clusterID := sqldb.GetClusterID(app, clusterType, clusterNS)
		cluster := s.cm.Create(ctx, &sqldb.CreateParams{
			ClusterID: clusterID,
			Memfs:     clusterType.Memfs(),
		})
		if _, err := cluster.Start(ctx, nil); err != nil {
			return err
		} else if err := cluster.Setup(ctx, params.AppRoot, parse.Meta); err != nil {
			return err
		}
		runProxy = func() error {
			return serveProxy(ctx, ln, func(ctx context.Context, client net.Conn) {
				_ = s.cm.PreauthProxyConn(client, clusterID)
			})
		}
	} else {
		proxy := &pgproxy.SingleBackendProxy{
			Log:             log.Logger,
			RequirePassword: false,
			FrontendTLS:     nil,
			DialBackend: func(ctx context.Context, startup *pgproxy.StartupData) (pgproxy.LogicalConn, error) {
				startupData, err := startup.Raw.Encode(nil)
				if err != nil {
					return nil, err
				}
				ws, err := platform.DBConnect(ctx, appID, params.EnvName, startup.Database, toRoleType(params.Role).String(), startupData)
				if err != nil {
					return nil, err
				}
				return &sqldb.WebsocketLogicalConn{Conn: ws}, nil
			},
		}

		runProxy = func() error {
			return proxy.Serve(ctx, ln)
		}
	}

	msgs := make(chan string, 10)
	defer close(msgs)
	go func() {
		for msg := range msgs {
			_ = stream.Send(&daemonpb.CommandMessage{Msg: &daemonpb.CommandMessage_Output{
				Output: &daemonpb.CommandOutput{
					Stdout: []byte(msg),
				},
			}})
		}
	}()

	return runProxy()
}

// DBReset resets the given databases, recreating them from scratch.
func (s *Server) DBReset(req *daemonpb.DBResetRequest, stream daemonpb.Daemon_DBResetServer) error {
	sendErr := func(err error) {
		_ = stream.Send(&daemonpb.CommandMessage{
			Msg: &daemonpb.CommandMessage_Output{Output: &daemonpb.CommandOutput{
				Stderr: []byte(err.Error() + "\n"),
			}},
		})
		_ = stream.Send(&daemonpb.CommandMessage{
			Msg: &daemonpb.CommandMessage_Exit{Exit: &daemonpb.CommandExit{
				Code: 1,
			}},
		})
	}

	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		sendErr(err)
		return nil
	}

	expSet, err := app.Experiments(nil)
	if err != nil {
		sendErr(err)
		return nil
	}

	// Parse the app to figure out what infrastructure is needed.
	bld := builderimpl.Resolve(app.Lang(), expSet)
	defer fns.CloseIgnore(bld)
	parse, err := bld.Parse(stream.Context(), builder.ParseParams{
		Build:       builder.DefaultBuildInfo(),
		App:         app,
		Experiments: expSet,
		WorkingDir:  ".",
		ParseTests:  false,
	})
	if err != nil {
		sendErr(err)
		return nil
	}

	clusterNS, err := s.namespaceOrActive(stream.Context(), app, req.Namespace)
	if err != nil {
		sendErr(err)
		return nil
	}

	clusterType := getClusterType(req)
	clusterID := sqldb.GetClusterID(app, clusterType, clusterNS)
	cluster, ok := s.cm.Get(clusterID)
	if !ok {
		cluster = s.cm.Create(stream.Context(), &sqldb.CreateParams{
			ClusterID: clusterID,
			Memfs:     clusterType.Memfs(),
		})
	}

	if _, err := cluster.Start(stream.Context(), nil); err != nil {
		sendErr(err)
		return nil
	}
	err = cluster.Recreate(stream.Context(), req.AppRoot, req.DatabaseNames, parse.Meta)
	if err != nil {
		sendErr(err)
	}
	return nil
}

func serveProxy(ctx context.Context, ln net.Listener, handler func(context.Context, net.Conn)) error {
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		frontend, e := ln.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("dbproxy: accept error: %v; retrying in %v", e, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return fmt.Errorf("dbproxy: could not accept: %w", e)
		}
		tempDelay = 0
		go handler(ctx, frontend)
	}
}

func getClusterType(req interface{ GetClusterType() daemonpb.DBClusterType }) sqldb.ClusterType {
	switch req.GetClusterType() {
	case daemonpb.DBClusterType_DB_CLUSTER_TYPE_RUN:
		return sqldb.Run
	case daemonpb.DBClusterType_DB_CLUSTER_TYPE_TEST:
		return sqldb.Test
	case daemonpb.DBClusterType_DB_CLUSTER_TYPE_SHADOW:
		return sqldb.Shadow
	default:
		return sqldb.Run
	}
}
