package dash

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/dash/dashproxy"
	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/internal/jsonrpc2"
	"encr.dev/internal/conf"
	"encr.dev/pkg/fns"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// NewServer starts a new server and returns it.
func NewServer(runMgr *run.Manager, tr trace2.Store, dashPort int) *Server {
	proxy, err := dashproxy.New(conf.DevDashURL)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create dash proxy")
	}

	s := &Server{
		proxy:    proxy,
		run:      runMgr,
		tr:       tr,
		dashPort: dashPort,
		traceCh:  make(chan *tracepb2.SpanSummary, 10),
		clients:  make(map[chan<- *notification]struct{}),
	}

	runMgr.AddListener(s)
	tr.Listen(s.traceCh)
	go s.listenTraces()
	return s
}

// Server is the http.Handler for serving the developer dashboard.
type Server struct {
	proxy    *httputil.ReverseProxy
	run      *run.Manager
	tr       trace2.Store
	dashPort int
	traceCh  chan *tracepb2.SpanSummary

	mu      sync.Mutex
	clients map[chan<- *notification]struct{}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/__encore":
		s.WebSocket(w, req)
	default:
		s.proxy.ServeHTTP(w, req)
	}
}

// WebSocket serves the jsonrpc2 API over WebSocket.
func (s *Server) WebSocket(w http.ResponseWriter, req *http.Request) {
	c, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Error().Err(err).Msg("dash: could not upgrade websocket")
		return
	}
	defer fns.CloseIgnore(c)
	log.Info().Msg("dash: websocket connection established")

	stream := &wsStream{c: c}
	conn := jsonrpc2.NewConn(stream)
	handler := &handler{rpc: conn, run: s.run, tr: s.tr}
	conn.Go(req.Context(), handler.Handle)

	ch := make(chan *notification, 20)
	s.addClient(ch)
	defer s.removeClient(ch)

	// nosemgrep: tools.semgrep-rules.semgrep-go.http-request-go-context
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

// hasClients reports whether there are any active clients.
func (s *Server) hasClients() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients) > 0
}

type notification struct {
	Method string
	Params interface{}
}

// notify notifies any active clients.
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
