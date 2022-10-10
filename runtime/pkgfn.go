//go:build encore_app

package encore

//publicapigen:drop
var Singleton *Manager

// Meta returns metadata about the running application.
//
// Meta will never return nil.
func Meta() *AppMetadata {
	return Singleton.Meta()
}

// CurrentRequest returns the Request that is currently being handled by the calling goroutine
//
// It is safe for concurrent use and will return a new Request on each evocation, so can be mutated by the
// calling code without impacting future calls.
//
// CurrentRequest never returns nil.
func CurrentRequest() *Request {
	return Singleton.CurrentRequest()
}
