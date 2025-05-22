package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	"encr.dev/cli/internal/jsonrpc2"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP (Message Context Provider) commands",
}

var (
	appID   string
	mcpPort int = 9900
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts an SSE based MCP session and prints the SSE URL",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		if appID == "" {
			appID = cmdutil.AppSlugOrLocalID()
		}
		setupDaemon(ctx)

		_, _ = fmt.Fprintf(os.Stderr, "  MCP Service is running!\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "  MCP SSE URL:        %s\n", aurora.Cyan(fmt.Sprintf(
			"http://localhost:%d/sse?app=%s", mcpPort, appID)))
		_, _ = fmt.Fprintf(os.Stderr, "  MCP stdio Command:  %s\n", aurora.Cyan(fmt.Sprintf(
			"encore mcp run --app=%s", appID)))
	},
}

type sseConnection struct {
	read  func() (typ, data string, err error)
	close func() error

	appID     string
	connected bool
	path      string
	client    *http.Client

	// Track outstanding request IDs
	mu         sync.Mutex
	requestIDs map[jsonrpc2.ID]struct{}
}

func (c *sseConnection) Read() (typ, data string, err error) {
	typ, data, err = c.read()
	if err != nil {
		c.connected = false
		return "", "", err
	}
	return typ, data, nil
}

func (c *sseConnection) Close() error {
	if c.close != nil {
		c.connected = false
		return c.close()
	}
	return nil
}

func (c *sseConnection) reconnect(ctx context.Context) error {
	// Close the existing connection if there is one
	if c.close != nil {
		_ = c.close()
	}
	c.connected = false

	// Initial backoff duration
	backoff := 1000 * time.Millisecond
	maxBackoff := 10 * time.Second

	for {
		// Check if context is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if root.Verbosity > 0 {
			fmt.Fprintf(os.Stderr, "Reconnecting to MCP: %v\n", backoff)
		}

		// Try to connect
		err := c.connect(ctx)
		if err == nil {
			c.connected = true
			return nil
		}

		// If connection failed, wait and retry with exponential backoff
		if root.Verbosity > 0 {
			fmt.Fprintf(os.Stderr, "Failed to connect to MCP: %v, retrying in %v\n", err, backoff)
		}

		select {
		case <-time.After(backoff):
			// Double the backoff for next attempt, but cap at maxBackoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *sseConnection) connect(ctx context.Context) error {
	setupDaemon(ctx)
	if c.client == nil {
		c.client = &http.Client{}
	}

	// Initialize the request IDs map
	c.mu.Lock()
	c.requestIDs = make(map[jsonrpc2.ID]struct{})
	c.mu.Unlock()

	resp, err := c.client.Get(fmt.Sprintf("http://localhost:%d/sse?app=%s", mcpPort, c.appID))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return fmt.Errorf("error getting session ID: %v", resp.Status)
	}
	c.read = eventReader(startLineReader(ctx, bufio.NewReader(resp.Body).ReadString))
	c.close = resp.Body.Close
	c.connected = true

	// Read the endpoint path
	event, path, err := c.Read()
	if err != nil {
		return fmt.Errorf("error reading endpoint path: %v", err)
	}
	if event != "endpoint" {
		return fmt.Errorf("expected endpoint event, got %q", event)
	}
	c.path = path

	return nil
}

func (c *sseConnection) SendMessage(data []byte) error {
	if !c.connected {
		return fmt.Errorf("not connected to MCP")
	}

	if c.client == nil {
		c.client = &http.Client{}
	}

	// Track the request ID if it's a Call
	msg, err := jsonrpc2.DecodeMessage(data)
	if err == nil {
		if call, ok := msg.(*jsonrpc2.Call); ok {
			c.mu.Lock()
			c.requestIDs[call.ID()] = struct{}{}
			c.mu.Unlock()
		}
	}

	resp, err := c.client.Post(fmt.Sprintf("http://localhost:%d%s", mcpPort, c.path), "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		return fmt.Errorf("error forwarding request: %v", resp.Status)
	}

	return nil
}

// CreateErrorResponse creates a JSON-RPC error response with the correct ID if available
func (c *sseConnection) CreateErrorResponse(id *jsonrpc2.ID, code int, message string) string {
	// Build the error response
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	// Include ID if available
	if id != nil {
		response["id"] = id

		// Remove from tracking as we're responding to it
		c.mu.Lock()
		delete(c.requestIDs, *id)
		c.mu.Unlock()
	} else {
		response["id"] = nil
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		// Fallback if marshaling fails
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":null,"error":{"code":%d,"message":"%s"}}`, code, message)
	}

	return string(jsonData)
}

// RemoveRequestID removes a request ID from tracking once a response is received
func (c *sseConnection) RemoveRequestID(id jsonrpc2.ID) {
	c.mu.Lock()
	delete(c.requestIDs, id)
	c.mu.Unlock()
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs an stdio-based MCP session",
	Run: func(cmd *cobra.Command, args []string) {

		ctx := cmd.Context()

		if appID == "" {
			appID = cmdutil.AppSlugOrLocalID()
		}

		if root.Verbosity > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "Starting an MCP session for app %s\n", appID)
		}

		conn := &sseConnection{appID: appID}
		if err := conn.connect(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to MCP: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		go func() {
			for {
				event, data, err := conn.Read()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading event: %v\n", err)

					conn.mu.Lock()
					requestIDs := maps.Clone(conn.requestIDs)
					conn.mu.Unlock()
					for id := range requestIDs {
						fmt.Println(conn.CreateErrorResponse(&id, -32700, "error"))
					}
					if err := conn.reconnect(ctx); err != nil {
						fmt.Fprintf(os.Stderr, "Error reconnecting to MCP: %v\n", err)
						os.Exit(1)
					}
					continue
				}
				if root.Verbosity > 0 {
					fmt.Fprintf(os.Stderr, "Received event: %s: %s\n", event, data)
				}
				if event == "message" {
					// If it's a response message, remove the ID from tracking
					responseMsg := struct {
						JSONRPC string       `json:"jsonrpc"`
						ID      *jsonrpc2.ID `json:"id"`
						Result  interface{}  `json:"result,omitempty"`
						Error   interface{}  `json:"error,omitempty"`
					}{}

					if err := json.Unmarshal([]byte(data), &responseMsg); err == nil && responseMsg.ID != nil {
						conn.RemoveRequestID(*responseMsg.ID)
					}

					fmt.Println(data)
				}
			}
		}()

		stdinReader := startLineReader(ctx, bufio.NewReader(os.Stdin).ReadBytes)
		if root.Verbosity > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "Listening on stdin for MCP requests\n\n")
		}

		for {
			line, err := stdinReader()
			if err != nil {
				if err == io.EOF || err == context.Canceled {
					break
				}
				fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
				os.Exit(1)
			}
			if strings.TrimSpace(string(line)) == "" {
				continue
			}

			msg, err := jsonrpc2.DecodeMessage(line)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error decoding request: %v\n", err)
				fmt.Println(conn.CreateErrorResponse(nil, -32700, "parse error"))
				continue
			}

			if root.Verbosity > 0 {
				fmt.Fprintf(os.Stderr, "Sending request: %s\n", line)
			}

			err = conn.SendMessage(line)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error sending message: %v\n", err)

				// Create error response with the request ID if available
				var requestID *jsonrpc2.ID
				if call, ok := msg.(*jsonrpc2.Call); ok {
					id := call.ID()
					requestID = &id
				}

				fmt.Println(conn.CreateErrorResponse(requestID, -32700, "error sending message"))
				continue
			}
		}
	},
}

type lineResult[T any] struct {
	res T
	err error
}

func startLineReader[T any](ctx context.Context, rd func(byte) (T, error)) func() (T, error) {
	channel := make(chan lineResult[T])
	go func() {
		for {
			line, err := rd('\n') // wait for Enter key
			if err != nil {
				channel <- lineResult[T]{err: err}
				return
			}
			channel <- lineResult[T]{res: line}
		}
	}()
	return func() (T, error) {
		var t T
		select {
		case <-ctx.Done():
			return t, ctx.Err()
		case result := <-channel:
			if result.err != nil {
				return t, result.err
			}
			return result.res, nil
		}
	}
}

func eventReader(reader func() (string, error)) func() (typ, data string, err error) {
	return func() (typ, data string, err error) {
		var line string
		for {
			line, err = reader()
			if err != nil {
				return "", "", err
			}
			if strings.HasPrefix(line, "event:") {
				break
			}
		}
		typ = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		line, err = reader()
		if err != nil {
			return "", "", err
		}
		if !strings.HasPrefix(line, "data:") {
			return "", "", fmt.Errorf("expected data: prefix, got %q", line)
		}
		data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		return typ, data, nil
	}
}

func init() {
	mcpCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&appID, "app", "", "The app ID to use for the MCP session")

	mcpCmd.AddCommand(startCmd)
	startCmd.Flags().StringVar(&appID, "app", "", "The app ID to use for the MCP session")

	root.Cmd.AddCommand(mcpCmd)
}
