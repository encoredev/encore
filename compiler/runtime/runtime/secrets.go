package runtime

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func LoadSecret(key string) string {
	val, ok := secrets[key]
	if !ok {
		fmt.Fprintln(os.Stderr, "encore: could not find secret", key)
		os.Exit(2)
	}
	return val
}

var secrets = loadSecrets()

func loadSecrets() map[string]string {
	const env = "ENCORE_SECRETS"
	encoded := os.Getenv(env)
	os.Unsetenv(env)
	if encoded == "" {
		return nil
	}

	// Format is "key1=val1,key2=val2" where values are base64-encoded using RawStdEncoding.
	secrets := make(map[string]string)
	fields := strings.Split(encoded, ",")
	for _, f := range fields {
		eql := strings.IndexByte(f, '=')
		if eql == -1 {
			fmt.Fprintln(os.Stderr, "encore: internal error: invalid ENCORE_SECRETS format")
			os.Exit(2)
		}
		key := f[:eql]
		value, err := base64.RawStdEncoding.DecodeString(f[eql+1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "encore: internal error: invalid ENCORE_SECRETS format")
			os.Exit(2)
		}
		secrets[key] = string(value)
	}
	return secrets
}
