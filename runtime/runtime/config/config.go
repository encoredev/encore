package config

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type Access string

const (
	Public  Access = "public"
	Auth    Access = "auth"
	Private Access = "private"
)

type Config struct {
	Static  *Static
	Runtime *Runtime
	Secrets map[string]string
}

type Static struct {
	Services []*Service
	// AuthData is the custom auth data type, or nil
	AuthData reflect.Type

	Testing     bool
	TestService string // service being tested, if any
}

type Service struct {
	Name      string
	RelPath   string // relative path to service pkg (from app root)
	Endpoints []*Endpoint
}

type Endpoint struct {
	Name    string
	Raw     bool
	Path    string
	Methods []string
	Access  Access
	Handler func(w http.ResponseWriter, req *http.Request, ps httprouter.Params)
}

type Runtime struct {
	AppID         string          `json:"app_id"`
	AppSlug       string          `json:"app_slug"`
	AppCommit     string          `json:"app_commit"`
	EnvID         string          `json:"env_id"`
	EnvName       string          `json:"env_name"`
	TraceEndpoint string          `json:"trace_endpoint"`
	AuthKeys      []EncoreAuthKey `json:"auth_keys"`
	SQLDatabases  []*SQLDatabase  `json:"sql_databases"`
}

func (r *Runtime) Copy() *Runtime {
	cfg := *r
	cfg.AuthKeys = make([]EncoreAuthKey, len(r.AuthKeys))
	for i, authKey := range r.AuthKeys {
		cfg.AuthKeys[i] = authKey.Copy()
	}
	copy(cfg.SQLDatabases, r.SQLDatabases)

	return &cfg
}

type EncoreAuthKey struct {
	KeyID uint32 `json:"kid"`
	Data  []byte `json:"data"`
}

func (eak EncoreAuthKey) Copy() EncoreAuthKey {
	c := eak
	copy(c.Data, eak.Data)
	return c
}

type SQLDatabase struct {
	EncoreName   string `json:"encore_name"`   // the Encore name for the database
	DatabaseName string `json:"database_name"` // the actual database name as known by the SQL server.
	Host         string `json:"host"`
	User         string `json:"user"`
	Password     string `json:"password"`
}

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

var Cfg *Config

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
