package runtime

import (
	"fmt"
	"os"

	"encore.dev/runtime/config"
)

func LoadSecret(key string) string {
	val, ok := config.Cfg.Secrets[key]
	if !ok {
		fmt.Fprintln(os.Stderr, "encore: could not find secret", key)
		os.Exit(2)
	}
	return val
}
