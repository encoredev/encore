package userconfig

// Config describes the configuration structure we support.
type Config struct {
	// Whether to open the Local Development Dashboard in the browser on `encore run`.
	// If set to "auto", the browser will be opened if the dashboard is not already open.
	RunBrowser string `koanf:"run.browser" oneof:"always,never,auto" default:"auto"`
}
