package auth

import "encore.dev/runtime"

// UID is a unique identifier representing a user (a user id).
type UID = runtime.UID

// UserID reports the uid of the user making the request.
// The second result is true if there is a user and false
// if the request was made without authentication details.
func UserID() (UID, bool) {
	req, _, ok := runtime.CurrentRequest()
	if ok {
		return req.UID, req.UID != ""
	}
	return "", false
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
func Data() interface{} {
	req, _, ok := runtime.CurrentRequest()
	if ok {
		return req.AuthData
	}
	return nil
}
