package config

type Runtime struct {
	AppID         string         `json:"app_id"`
	AppSlug       string         `json:"app_slug"`
	EnvID         string         `json:"env_id"`
	EnvName       string         `json:"env_name"`
	TraceEndpoint string         `json:"trace_endpoint"`
	SQLDatabases  []*SQLDatabase `json:"sql_databases"`
}

type SQLDatabase struct {
	EncoreName   string `json:"encore_name"`   // the Encore name for the database
	DatabaseName string `json:"database_name"` // the actual database name as known by the SQL server.
	Host         string `json:"host"`
	User         string `json:"user"`
	Password     string `json:"password"`
}
