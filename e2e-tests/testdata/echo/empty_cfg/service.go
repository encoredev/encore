package emptycfg

import (
	"context"

	"encore.dev/config"
)

type cfg struct {

}

var Config = config.Load[*cfg]()

//encore:api public
func AnAPI(ctx context.Context) error {
	return nil
}
