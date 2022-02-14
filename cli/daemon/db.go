package daemon

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"encr.dev/cli/daemon/internal/manifest"
	"encr.dev/cli/daemon/internal/runlog"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/appfile"
	"encr.dev/cli/internal/platform"
	"encr.dev/pkg/pgproxy"
	daemonpb "encr.dev/proto/encore/daemon"
)

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
	port, passwd, err := sqldb.OneshotProxy(appID, req.EnvName)
	if err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("postgresql://encore:%s@localhost:%d/%s?sslmode=disable", passwd, port, req.DbName)
	return &daemonpb.DBConnectResponse{Dsn: dsn}, nil
}

func (s *Server) dbConnectLocal(ctx context.Context, req *daemonpb.DBConnectRequest) (*daemonpb.DBConnectResponse, error) {
	// Parse the app to figure out what infrastructure is needed.
	parse, err := s.parseApp(req.AppRoot, ".", false)
	if err != nil {
		return nil, err
	}

	man, err := manifest.ReadOrCreate(req.AppRoot)
	if err != nil {
		return nil, err
	}

	clusterID := man.AppID
	log := log.With().Str("appID", man.AppID).Logger()
	log.Info().Msg("setting up database cluster")
	cluster := s.cm.Init(ctx, &sqldb.InitParams{
		ClusterID: clusterID,
		Meta:      parse.Meta,
		Memfs:     false,
	})
	// TODO would be nice to stream this to the CLI
	if err := cluster.Start(runlog.OS()); err != nil {
		log.Error().Err(err).Msg("failed to start db cluster")
		return nil, err
	} else if err := cluster.Create(ctx, req.AppRoot, parse.Meta); err != nil {
		log.Error().Err(err).Msg("failed to create databases")
		return nil, err
	}
	log.Info().Msg("created database cluster")
	dsn := fmt.Sprintf("postgresql://encore:%s@localhost:%d/%s?sslmode=disable", clusterID, s.mgr.DBProxyPort, req.DbName)
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
		ln.Close()
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
		// Parse the app to figure out what infrastructure is needed.
		parse, err := s.parseApp(params.AppRoot, ".", false)
		if err != nil {
			return err
		}

		man, err := manifest.ReadOrCreate(params.AppRoot)
		if err != nil {
			return err
		}

		clusterID := man.AppID
		cluster := s.cm.Init(ctx, &sqldb.InitParams{
			ClusterID: clusterID,
			Meta:      parse.Meta,
			Memfs:     false,
		})
		if err := cluster.Start(&streamLog{stream: stream, buffered: false}); err != nil {
			return err
		} else if err := cluster.Create(ctx, params.AppRoot, parse.Meta); err != nil {
			return err
		}
		runProxy = func() error {
			return serveProxy(ctx, ln, func(ctx context.Context, client net.Conn) {
				s.cm.PreauthProxyConn(client, clusterID)
			})
		}
	} else {
		proxy := &pgproxy.SingleBackendProxy{
			Log:             log.Logger,
			RequirePassword: false,
			FrontendTLS:     nil,
			DialBackend: func(ctx context.Context, startup *pgproxy.StartupData) (pgproxy.LogicalConn, error) {
				startupData := startup.Raw.Encode(nil)
				ws, err := platform.DBConnect(ctx, appID, params.EnvName, startup.Database, startupData)
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
			stream.Send(&daemonpb.CommandMessage{Msg: &daemonpb.CommandMessage_Output{
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
		stream.Send(&daemonpb.CommandMessage{
			Msg: &daemonpb.CommandMessage_Output{Output: &daemonpb.CommandOutput{
				Stderr: []byte(err.Error() + "\n"),
			}},
		})
		stream.Send(&daemonpb.CommandMessage{
			Msg: &daemonpb.CommandMessage_Exit{Exit: &daemonpb.CommandExit{
				Code: 1,
			}},
		})
	}

	// Parse the app to figure out what infrastructure is needed.
	parse, err := s.parseApp(req.AppRoot, ".", false)
	if err != nil {
		sendErr(err)
		return nil
	}

	man, err := manifest.ReadOrCreate(req.AppRoot)
	if err != nil {
		sendErr(err)
		return nil
	}

	clusterID := man.AppID
	cluster, ok := s.cm.Get(clusterID)
	if !ok {
		cluster = s.cm.Init(stream.Context(), &sqldb.InitParams{
			ClusterID: clusterID,
			Memfs:     false,
			Meta:      parse.Meta,
		})
	}

	if err := cluster.Start(&streamLog{stream: stream, buffered: false}); err != nil {
		sendErr(err)
		return nil
	}

	err = cluster.Recreate(stream.Context(), req.AppRoot, req.Services, parse.Meta)
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
