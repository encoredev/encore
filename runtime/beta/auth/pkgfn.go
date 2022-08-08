//go:build encore_app

package auth

import (
	// Ensure the runtime is initialized.
	_ "encore.dev/appruntime/app/appinit"
)

//publicapigen:drop
var Singleton *Manager // injected on app init

// UserID reports the uid of the user making the request.
// The second result is true if there is a user and false
// if the request was made without authentication details.
func UserID() (UID, bool) {
	return Singleton.UserID()
}

// Data returns the structured auth data for the request.
// It returns nil if the request was made without authentication details,
// and the API endpoint does not require them.
//
// Expected usage is to immediately cast it to the registered auth data type:
//
//   usr, ok := auth.Data().(*user.Data)
//   if !ok { /* ... */ }
//
func Data() any {
	return Singleton.Data()
}
