package encoreenv

import (
	"os"
	"strings"
)

func Get(env string) string {
	return envs[env]
}

func Set(env, val string) {
	envs[env] = val
}

var envs map[string]string

func init() {
	envs = make(map[string]string)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ENCORE_") {
			key, val, _ := strings.Cut(e, "=")
			envs[key] = val
			os.Unsetenv(key)
		}
	}
}
