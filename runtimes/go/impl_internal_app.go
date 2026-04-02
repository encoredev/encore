//go:build encore_app

package encore

import (
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/reqtrack"
)

//publicapigen:drop
var Singleton = NewManager(appconf.Static, appconf.Runtime, reqtrack.Singleton)

func meta() *AppMetadata {
	return Singleton.Meta()
}

func currentRequest() *Request {
	return Singleton.CurrentRequest()
}
