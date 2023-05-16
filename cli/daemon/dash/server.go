package dash

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/internal/jsonrpc2"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

//go:embed dashapp/dist/*
var assets embed.FS

// NewServer starts a new server and returns it.
func NewServer(runMgr *run.Manager, tr trace2.Store) *Server {
	assets, err := fs.Sub(assets, "dashapp/dist")
	if err != nil {
		log.Fatal().Err(err).Msg("could not get dash assets")
	}

	s := &Server{
		run:     runMgr,
		tr:      tr,
		assets:  assets,
		traceCh: make(chan *tracepb2.SpanSummary, 10),
		clients: make(map[chan<- *notification]struct{}),
	}

	runMgr.AddListener(s)
	tr.Listen(s.traceCh)
	go s.listenTraces()
	return s
}

// Server is the http.Handler for serving the developer dashboard.
type Server struct {
	run     *run.Manager
	tr      trace2.Store
	traceCh chan *tracepb2.SpanSummary
	assets  fs.FS

	mu      sync.Mutex
	clients map[chan<- *notification]struct{}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	fs := http.FileServer(http.FS(s.assets))
	switch {
	case path == "/__encore":
		s.WebSocket(w, req)
	case strings.HasPrefix(path, "/assets/") || path == "/favicon.ico":
		// We've seen cases where net/http's content type detection gets it wrong
		// for the bundled javascript files. Work around it by specifying it manually.
		if filepath.Ext(path) == ".js" {
			w.Header().Set("Content-Type", "application/javascript")
		}
		fs.ServeHTTP(w, req)
	default:
		// Serve the index page for all other paths since we use client-side routing.
		req.URL.Path = "/"
		fs.ServeHTTP(w, req)
	}
}

// WebSocket serves the jsonrpc2 API over WebSocket.
func (s *Server) WebSocket(w http.ResponseWriter, req *http.Request) {
	c, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Error().Err(err).Msg("dash: could not upgrade websocket")
		return
	}
	defer c.Close()
	log.Info().Msg("dash: websocket connection established")

	stream := &wsStream{c: c}
	conn := jsonrpc2.NewConn(stream)
	handler := &handler{rpc: conn, run: s.run, tr: s.tr}
	conn.Go(req.Context(), handler.Handle)

	ch := make(chan *notification, 20)
	s.addClient(ch)
	defer s.removeClient(ch)
	go handler.listenNotify(req.Context(), ch)

	<-conn.Done()
	if err := conn.Err(); err != nil {
		if ce, ok := err.(*websocket.CloseError); ok && ce.Code == websocket.CloseNormalClosure {
			log.Info().Msg("dash: websocket closed")
		} else {
			log.Info().Err(err).Msg("dash: websocket closed with error")
		}
	}
}

func (s *Server) addClient(ch chan *notification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[ch] = struct{}{}
}

func (s *Server) removeClient(ch chan *notification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, ch)
}

type notification struct {
	Method string
	Params interface{}
}

func (s *Server) notify(n *notification) {
	var clients []chan<- *notification
	s.mu.Lock()
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	for _, c := range clients {
		select {
		case c <- n:
		default:
		}
	}
}

// wsStream implements jsonrpc2.Stream over a websocket.
type wsStream struct {
	writeMu sync.Mutex
	c       *websocket.Conn
}

func (s *wsStream) Close() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.c.Close()
}

func (s *wsStream) Read(context.Context) (jsonrpc2.Message, int64, error) {
	typ, data, err := s.c.ReadMessage()
	if err != nil {
		return nil, 0, err
	}
	if typ != websocket.TextMessage {
		return nil, 0, fmt.Errorf("webedit.wsStream: got non-text message type %v", typ)
	}
	msg, err := jsonrpc2.DecodeMessage(data)
	if err != nil {
		return nil, 0, err
	}
	return msg, int64(len(data)), nil
}

func (s *wsStream) Write(ctx context.Context, msg jsonrpc2.Message) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	data, err := json.Marshal(msg)
	if err != nil {
		return 0, err
	}
	err = s.c.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return 0, err
	}
	return int64(len(data)), nil
}
