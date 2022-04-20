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
}

type EncoreAuthKey struct {
	KeyID uint32 `json:"kid"`
	Data  []byte `json:"data"`
}

type SQLDatabase struct {
	EncoreName   string `json:"encore_name"`   // the Encore name for the database
	DatabaseName string `json:"database_name"` // the actual database name as known by the SQL server.
	Host         string `json:"host"`
	User         string `json:"user"`
	Password     string `json:"password"`
}
