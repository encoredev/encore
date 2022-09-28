package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"encr.dev/cli/internal/appfile"
	"encr.dev/cli/internal/platform"
)

var (
	logsEnv  string
	logsJSON bool
)

var logsCmd = &cobra.Command{
	Use:   "logs [--env=prod] [--json]",
	Short: "Streams logs from your application",

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		appRoot, _ := determineAppRoot()
		streamLogs(appRoot, logsEnv)
	},
}

func streamLogs(appRoot, envName string) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	appSlug, err := appfile.Slug(appRoot)
	if err != nil {
		fatal(err)
	} else if appSlug == "" {
		fatal("app is not linked with Encore Cloud")
	}

	if envName == "" {
		envName = "@primary"
	}
	logs, err := platform.EnvLogs(ctx, appSlug, envName)
	if err != nil {
		var e platform.Error
		if errors.As(err, &e) {
			switch e.Code {
			case "env_not_found":
				fatalf("environment %q not found", envName)
			}
		}
		fatal(err)
	}

	go func() {
		<-ctx.Done()
		logs.Close()
	}()

	const (
		// Time allowed to write a message to the peer.
		writeWait = 10 * time.Second

		// Time allowed to read the next pong message from the peer.
		pongTimeout = 60 * time.Second

		// Send pings to peer with this period. Must be less than pongWait.
		pingPeriod = (pongTimeout * 9) / 10
	)

	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		logs.Close()
	}()

	go func() {
		defer logs.Close() // close the stream if we fail to ping the server.
		for range pingTicker.C {
			logs.SetWriteDeadline(time.Now().Add(writeWait))
			if err := logs.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	logs.SetReadDeadline(time.Now().Add(pongTimeout))
	logs.SetPongHandler(func(string) error { logs.SetReadDeadline(time.Now().Add(pongTimeout)); return nil })

	// Use the same configuration as the runtime
	zerolog.TimeFieldFormat = time.RFC3339Nano

	cw := zerolog.NewConsoleWriter()
	for {
		_, message, err := logs.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fatal("the server closed the connection unexpectedly.")
			}
			return
		}

		lines := bytes.Split(message, []byte("\n"))
		for _, line := range lines {
			// Pretty-print logs if requested and it looks like a JSON log line
			if !logsJSON && bytes.HasPrefix(line, []byte{'{'}) {
				if _, err := cw.Write(line); err != nil {
					// Fall back to regular stdout in case of error
					os.Stdout.Write(line)
					os.Stdout.Write([]byte("\n"))
				}
			} else {
				os.Stdout.Write(line)
				os.Stdout.Write([]byte("\n"))
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().StringVarP(&logsEnv, "env", "e", "", "Environment name to stream logs from (defaults to the primary environment)")
	logsCmd.Flags().BoolVar(&logsJSON, "json", false, "Whether to print logs in raw JSON format")
}
