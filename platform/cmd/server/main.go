package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encoru.dev/platform/internal/config"
	"encoru.dev/platform/internal/server"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	app := server.New(&log.Logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		log.Info().Msg("shutting down...")
		if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
			log.Error().Err(err).Msg("shutdown error")
		}
	}()

	addr := cfg.Host + ":" + cfg.Port
	log.Info().Str("addr", addr).Msg("starting platform")

	if err := app.Listen(addr); err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
}
