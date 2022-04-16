package config

// NOTE: This file should be kept in sync between runtime/config and cli/daemon/runtime/config.

type Runtime struct {
	AppID         string          `json:"app_id"`
	AppSlug       string          `json:"app_slug"`
	AppCommit     CommitInfo      `json:"app_commit"`
	APIBaseURL    string          `json:"api_base_url"`
	EnvID         string          `json:"env_id"`
	EnvName       string          `json:"env_name"`
	EnvType       string          `json:"env_type"`
	TraceEndpoint string          `json:"trace_endpoint"`
	AuthKeys      []EncoreAuthKey `json:"auth_keys"`
	SQLDatabases  []*SQLDatabase  `json:"sql_databases"`
}

type CommitInfo struct {
	Revision    string `json:"revision"`
	Uncommitted bool   `json:"uncommitted"`
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
