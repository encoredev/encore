package config

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"strings"
)

// ParseRuntime parses the Encore runtime config.
func ParseRuntime(s string) *Runtime {
	if s == "" {
		log.Fatalln("encore runtime: fatal error: no encore runtime config provided")
	}

	// We used to support RawURLEncoding, but now we use StdEncoding.
	// Try both if StdEncoding fails.
	var (
		bytes []byte
		err   error
	)
	if bytes, err = base64.StdEncoding.DecodeString(s); err != nil {
		bytes, err = base64.RawURLEncoding.DecodeString(s)
	}
	if err != nil {
		log.Fatalln("encore runtime: fatal error: could not decode encore runtime config:", err)
	}

	var cfg Runtime
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		log.Fatalln("encore runtime: fatal error: could not parse encore runtime config:", err)
	}

	if _, err := url.Parse(cfg.APIBaseURL); err != nil {
		log.Fatalln("encore runtime: fatal error: could not parse api base url from encore runtime config:", err)
	}

	if deployID := os.Getenv("ENCORE_DEPLOY_ID"); deployID != "" {
		cfg.DeployID = deployID
	}

	return &cfg
}

// ParseSecrets parses secrets in "key1=base64(val1),key2=base64(val2)" format into a map.
func ParseSecrets(s string) map[string]string {
	m := make(map[string]string)
	if s == "" {
		return m
	}
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			log.Fatalln("encore runtime: fatal error: invalid secret value")
		}
		val, err := base64.RawURLEncoding.DecodeString(kv[1])
		if err != nil {
			log.Fatalln("encore runtime: fatal error: invalid secret value")
		}
		m[kv[0]] = string(val)
	}
	return m
}

// GetAndClearEnv gets an env variable by name and then clears it.
func GetAndClearEnv(env string) string {
	val := os.Getenv(env)
	_ = os.Unsetenv(env)
	return val
}
