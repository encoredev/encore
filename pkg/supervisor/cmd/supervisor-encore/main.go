package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"os"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"

	"encr.dev/pkg/supervisor"
	runtimev1 "encr.dev/proto/encore/runtime/v1"
)

func main() {
	log.Info().Msg("supervisor starting")
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("unable to run supervisor")
	}
}

func run() error {
	cfgPath := flag.String("c", "", "path to the config file")
	flag.Parse()
	if *cfgPath == "" {
		return errors.New("missing config file")
	}

	cfg, err := loadSupervisorConfig(*cfgPath)
	if err != nil {
		return errors.Wrap(err, "load supervisor config")
	}

	// Configure the logger.
	rtcfg, err := loadRuntimeConfig()
	if err != nil {
		return errors.Wrap(err, "load runtime config")
	}
	configureLogger(rtcfg)

	super, err := supervisor.New(cfg, rtcfg)
	if err != nil {
		return errors.Wrap(err, "create supervisor")
	}

	err = super.Run(context.Background())
	return errors.Wrap(err, "run supervisor")
}

func loadSupervisorConfig(path string) (*supervisor.Config, error) {
	var cfg supervisor.Config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read config file")
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, errors.Wrap(err, "unmarshal config file")
	}
	return &cfg, nil
}

// loadRuntimeConfig loads the runtime config from the ENCORE_RUNTIME_CONFIG env var.
func loadRuntimeConfig() (*runtimev1.RuntimeConfig, error) {
	val, ok := os.LookupEnv("ENCORE_RUNTIME_CONFIG")
	if !ok {
		return nil, errors.New("ENCORE_RUNTIME_CONFIG not set")
	}

	decoded, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode ENCORE_RUNTIME_CONFIG")
	}

	var cfg runtimev1.RuntimeConfig
	if err := proto.Unmarshal(decoded, &cfg); err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal runtime config")
	}

	return &cfg, nil
}

func configureLogger(cfg *runtimev1.RuntimeConfig) {
	// Log in GCP's log format for Encore Cloud and GCP.
	if cloud := cfg.Environment.Cloud; cloud == runtimev1.Environment_CLOUD_GCP || cloud == runtimev1.Environment_CLOUD_ENCORE {
		zerolog.LevelFieldName = "severity"
		zerolog.TimestampFieldName = "timestamp"
		zerolog.TimeFieldFormat = time.RFC3339Nano
	}

	// Create our root logger
	logger := zerolog.New(os.Stderr).
		Level(zerolog.DebugLevel).
		With().Caller().Timestamp().Stack().Str("process", "supervisor").
		Logger()

	log.Logger = logger
}
