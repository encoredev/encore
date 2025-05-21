package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs an stdio-based MCP session",
	Run: func(cmd *cobra.Command, args []string) {

		ctx := cmd.Context()

		if appID == "" {
			appID = cmdutil.AppSlugOrLocalID()
		}

		setupDaemon(ctx)

		if root.Verbosity > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "Starting an MCP session for app %s\n", appID)
		}

		client := &http.Client{}
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/sse?app=%s", mcpPort, appID))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting session ID: %v\n", err)
			os.Exit(1)
		}
		if resp.StatusCode != 200 {
			fmt.Fprintf(os.Stderr, "Error getting session ID: %v\n", resp.Status)
			os.Exit(1)
		}
		defer resp.Body.Close()

		reader := eventReader(startLineReader(ctx, bufio.NewReader(resp.Body).ReadString))
		event, path, err := reader()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading event: %v\n", err)
			os.Exit(1)
		}
		if event != "endpoint" {
			fmt.Fprintf(os.Stderr, "Expected endpoint event, got %q\n", event)
			os.Exit(1)
		}

		go func() {
			for {
				event, data, err := reader()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading event: %v\n", err)
					os.Exit(1)
				}
				if root.Verbosity > 0 {
					fmt.Fprintf(os.Stderr, "Received event: %s: %s\n", event, data)
				}
				if event == "message" {
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
			if _, err = jsonrpc2.DecodeMessage(line); err != nil {
				fmt.Fprintf(os.Stderr, "Error decoding request: %v\n", err)
				continue
			}
			if root.Verbosity > 0 {
				fmt.Fprintf(os.Stderr, "Sending request: %s\n", line)
			}
			resp, err := client.Post(fmt.Sprintf("http://localhost:%d%s", mcpPort, path), "application/json", bytes.NewReader(line))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error forwarding request: %v\n", err)
				continue
			}
			if resp.StatusCode != 202 {
				fmt.Fprintf(os.Stderr, "Error forwarding request: %v\n", resp.Status)
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
