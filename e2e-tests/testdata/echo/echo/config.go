package echo

import (
	"context"

	"encore.dev/config"
)

type CfgType[T any] struct {
	ReadOnlyMode config.Bool
	PublicKey    config.Bytes
	AdminUsers   config.Values[string]

	SubConfig config.Value[struct {
		SubKey *SubCfgType[T]
	}]

	Currencies map[string]struct {
		Name    config.String
		Code    config.String
		Aliases config.Values[string]
	}

	AnotherList config.Values[struct {
		Name config.String
	}]
}

type SubCfgType[T any] struct {
	MaxCount T
}

var cfg = config.Load[*CfgType[uint]]()

type ConfigResponse struct {
	ReadOnlyMode bool
	PublicKey    []byte
	SubKeyCount  uint
	AdminUsers   []string
}

//encore:api public
func ConfigValues(ctx context.Context) (ConfigResponse, error) {
	return ConfigResponse{
		ReadOnlyMode: cfg.ReadOnlyMode(),
		PublicKey:    cfg.PublicKey(),
		AdminUsers:   cfg.AdminUsers(),
		SubKeyCount:  cfg.SubConfig().SubKey.MaxCount,
	}, nil
}
