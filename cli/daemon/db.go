package daemon

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"encr.dev/cli/daemon/internal/appfile"
	"encr.dev/cli/daemon/internal/manifest"
	"encr.dev/cli/daemon/internal/runlog"
	"encr.dev/cli/daemon/sqldb"
	daemonpb "encr.dev/proto/encore/daemon"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	port, passwd, err := sqldb.OneshotProxy(s.rc, appID, req.EnvName)
	if err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("postgresql://encore:%s@localhost:%d/%s?sslmode=disable", passwd, port, req.SvcName)
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
	dsn := fmt.Sprintf("postgresql://encore:%s@localhost:%d/%s?sslmode=disable", clusterID, s.mgr.DBProxyPort, req.SvcName)
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
	defer func() {
		if err != nil {
			ln.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

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

	var handler func(context.Context, net.Conn)
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
		if err := cluster.Start(&streamLog{stream: stream}); err != nil {
			return err
		} else if err := cluster.Create(ctx, params.AppRoot, parse.Meta); err != nil {
			return err
		}
		handler = func(ctx context.Context, frontend net.Conn) {
			s.cm.PreauthProxyConn(frontend, clusterID)
		}
	} else {
		handler = func(ctx context.Context, frontend net.Conn) {
			sqldb.ProxyRemoteConn(ctx, s.rc, frontend, "", appID, params.EnvName)
		}
	}

	msgs := make(chan string, 10)
	go func() {
		for msg := range msgs {
			stream.Send(&daemonpb.CommandMessage{Msg: &daemonpb.CommandMessage_Output{
				Output: &daemonpb.CommandOutput{
					Stdout: []byte(msg),
				},
			}})
		}
	}()

	var wg sync.WaitGroup
	err = serveProxy(ctx, ln, func(ctx context.Context, frontend net.Conn) {
		wg.Add(1)
		defer wg.Done()
		msgs <- "dbproxy: connection opened\n"
		handler(ctx, frontend)
		msgs <- "dbproxy: connection closed\n"
	})

	go func() {
		// Close the msgs chan when all connections are closed
		wg.Wait()
		close(msgs)
	}()

	return err
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

	if err := cluster.Start(&streamLog{stream: stream}); err != nil {
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
