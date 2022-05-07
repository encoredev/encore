package config

type Runtime struct {
	AppID         string          `json:"app_id"`
	AppSlug       string          `json:"app_slug"`
	AppCommit     string          `json:"app_commit"`
	EnvID         string          `json:"env_id"`
	EnvName       string          `json:"env_name"`
	TraceEndpoint string          `json:"trace_endpoint"`
	AuthKeys      []EncoreAuthKey `json:"auth_keys"`
	SQLDatabases  []*SQLDatabase  `json:"sql_databases"`
	SQLServers    []*SQLServer    `json:"sql_servers"`
}

type EncoreAuthKey struct {
	KeyID uint32 `json:"kid"`
	Data  []byte `json:"data"`
}

type SQLServer struct {
	// Host is the host to connect to.
	// Valid formats are "hostname", "hostname:port", and "/path/to/unix.socket".
	Host string `json:"host"`

	// MinConnections is the minimum number of open connections to use
	// for this database. If zero it defaults to 2.
	MinConnections int `json:"min_connections"`

	// MaxConnections is the maximum number of open connections to use
	// for this database. If zero it defaults to 30.
	MaxConnections int `json:"max_connections"`

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
}
