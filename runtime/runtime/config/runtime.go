package config

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/url"
	"strings"
)

func init() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("encore runtime: fatal error: could not load config: %v", err)
	}
	Cfg = cfg
}

// loadConfig loads the Encore app configuration.
// It is provided by the main package using go:linkname.
func loadConfig() (*Config, error)

// ParseRuntime parses the Encore runtime config.
func ParseRuntime(s string) *Runtime {
	if s == "" {
		log.Fatalln("encore runtime: fatal error: no encore runtime config provided")
	}
	bytes, err := base64.RawURLEncoding.DecodeString(s)
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

// JsonIndentStepForResponses is the number of spaces to indent JSON responses sent from the application.
//
// - 0 means no pretty printing will occur for JSON responses.
// - any other value means pretty printing with that number of spaces for each level of indentation.
//
// In production environments this function will return 0, for all others it will return 2.
func JsonIndentStepForResponses() int {
	if Cfg.Runtime.EnvType == "production" {
		return 0
	}

	return 2
}
