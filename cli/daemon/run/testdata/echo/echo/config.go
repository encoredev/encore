package echo

import (
	"context"

	"encore.dev/config"
)

type CfgType struct {
	ReadOnlyMode config.Bool
	PublicKey    config.Bytes
	AdminUsers   config.Values[string]

	SubConfig config.Value[struct {
		SubKey SubCfgType
	}]
}

type SubCfgType struct {
	MaxCount uint
}

var cfg = config.Load[CfgType]()

type ConfigResponse struct {
	ReadOnlyMode bool
	PublicKey    []byte
	SubKeyCount  uint
	AdminUsers   []string
}

//encore:api public
func ConfigValues(ctx context.Context) (ConfigResponse, error) {
	return ConfigResponse {
		ReadOnlyMode: cfg.ReadOnlyMode(),
		PublicKey:    cfg.PublicKey(),
		AdminUsers:   cfg.AdminUsers(),
		SubKeyCount:  cfg.SubConfig().SubKey.MaxCount,
	}, nil
}
