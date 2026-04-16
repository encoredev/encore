package config

import "github.com/caarlos0/env/v11"

type Config struct {
	Host     string `env:"PLATFORM_HOST"      envDefault:"0.0.0.0"`
	Port     string `env:"PLATFORM_PORT"      envDefault:"8080"`
	LogLevel string `env:"PLATFORM_LOG_LEVEL" envDefault:"info"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
