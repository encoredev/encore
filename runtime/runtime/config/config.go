package config

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

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

	// The version of Encore which the app was compiled with.
	// This is string is for informational use only, and it's format should not be relied on.
	EncoreCompiler string
	AppCommit      CommitInfo // The commit which this service was built from

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
	APIBaseURL    string          `json:"api_base_url"`
	EnvID         string          `json:"env_id"`
	EnvName       string          `json:"env_name"`
	EnvType       string          `json:"env_type"`
	EnvCloud      string          `json:"env_cloud"`
	DeployID      string          `json:"deploy_id"`
	DeployedAt    time.Time       `json:"deploy_time"`
	TraceEndpoint string          `json:"trace_endpoint"`
	AuthKeys      []EncoreAuthKey `json:"auth_keys"`
	SQLDatabases  []*SQLDatabase  `json:"sql_databases"`
	SQLServers    []*SQLServer    `json:"sql_servers"`
	CORS          *CORS           `json:"cors"`

	// ShutdownTimeout is the duration before non-graceful shutdown is initiated,
	// meaning connections are closed even if outstanding requests are still in flight.
	// If zero, it shuts down immediately.
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

type CORS struct {
	// DisableCredentials, if true, causes Encore to respond to OPTIONS requests
	// without setting Access-Control-Allow-Credentials: true.
	DisableCredentials bool `json:"disable_credentials"`

	// AllowOriginsWithCredentials specifies the allowed origins for requests
	// that include credentials. If a request is made from an Origin in this list
	// Encore responds with Access-Control-Allow-Origin: <Origin>.
	// If DisableCredentials is true this field is not used.
	AllowOriginsWithCredentials []string `json:"allow_origins_with_credentials"`

	// AllowOriginsWithoutCredentials specifies the allowed origins for requests
	// that don't include credentials. If nil it defaults to allowing all domains
	// (equivalent to []string{"*"}).
	AllowOriginsWithoutCredentials []string `json:"allow_origins_without_credentials"`
}

type CommitInfo struct {
	Revision    string `json:"revision"`
	Uncommitted bool   `json:"uncommitted"`
}

func (ci CommitInfo) AsRevisionString() string {
	if ci.Uncommitted {
		return fmt.Sprintf("%s-modified", ci.Revision)
	}
	return ci.Revision
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

type SQLServer struct {
	// Host is the host to connect to.
	// Valid formats are "hostname", "hostname:port", and "/path/to/unix.socket".
	Host string `json:"host"`

	// ServerCACert is the PEM-encoded server CA cert, or "" if not required.
	ServerCACert string `json:"server_ca_cert"`
	// ClientCert is the PEM-encoded client cert, or "" if not required.
	ClientCert string `json:"client_cert"`
	// ClientKey is the PEM-encoded client key, or "" if not required.
	ClientKey string `json:"client_key"`
}

type SQLDatabase struct {
	ServerID     int    `json:"server_id"`     // the index into (*Runtime).SQLServers
	EncoreName   string `json:"encore_name"`   // the Encore name for the database
	DatabaseName string `json:"database_name"` // the actual database name as known by the SQL server.
	User         string `json:"user"`
	Password     string `json:"password"`

	// MinConnections is the minimum number of open connections to use
	// for this database. If zero it defaults to 2.
	MinConnections int `json:"min_connections"`

	// MaxConnections is the maximum number of open connections to use
	// for this database. If zero it defaults to 30.
	MaxConnections int `json:"max_connections"`
}

var Cfg *Config
