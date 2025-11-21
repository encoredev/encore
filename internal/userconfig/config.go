package userconfig

// Config describes the configuration structure we support.
type Config struct {
	// Whether to open the Local Development Dashboard in the browser on `encore run`.
	// If set to "auto", the browser will be opened if the dashboard is not already open.
	RunBrowser string `koanf:"run.browser" oneof:"always,never,auto" default:"auto"`

	// Always choose this tool when creating an app or when initializing llm tools
	// for an existing app, unless overriden via --llm-rules flag on command line.
	LLMRules string `koanf:"llm_rules" oneof:",cursor,claudcode,vscode,agentsmd,zed" default:""`
}
