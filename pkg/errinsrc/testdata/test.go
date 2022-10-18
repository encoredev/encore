package testdata

import (
	"context"

	"encore.dev/config"
)

type Config struct {
	Blah struct {
		Foo config.Bool `json:"foo"`
		Bar config.Int  `json:"bar"`
	} `json:"blah"`
}

var cfg = config.Load[Config]()

type WhatIsBarResponse struct {
	Bar int
}

//encore:api public
func WhatIsBar(ctx context.Context) (*WhatIsBarResponse, error) {
	return &WhatIsBarResponse{
		Bar: cfg.Blah.Bar(),
	}, nil
}
