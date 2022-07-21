// Package auth provides the APIs to get information about the authenticated users.
//
// For more information about how authentication works with Encore applications see https://encore.dev/docs/develop/auth.
package auth

import (
	"context"

	"encore.dev/appruntime/api"
	"encore.dev/appruntime/model"
	"encore.dev/appruntime/reqtrack"
)

// UID is a unique identifier representing a user (a user id).
type UID = model.UID

type Manager struct {
	rt *reqtrack.RequestTracker
}

func NewManager(rt *reqtrack.RequestTracker) *Manager {
	return &Manager{rt}
}

// UserID reports the uid of the user making the request.
// The second result is true if there is a user and false
// if the request was made without authentication details.
func (mgr *Manager) UserID() (UID, bool) {
	if curr := mgr.rt.Current(); curr.Req != nil {
		return curr.Req.UID, curr.Req.UID != ""
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
func (mgr *Manager) Data() interface{} {
	if curr := mgr.rt.Current(); curr.Req != nil {
		return curr.Req.AuthData
	}
	return nil
}

// WithContext returns a new context that sets the auth information for outgoing API calls.
// It does not affect the auth information for the current request.
//
// Passing in an empty string as the uid results in unsetting the auth information,
// causing future API calls to behave as if there was no authenticated user.
//
// If the application's auth handler returns custom auth data, two additional
// requirements exist. First, the data parameter passed to WithContext must be of
// the same type as the auth handler returns. Second, if the uid argument is not
// the empty string then data may not be nil. If these requirements are not met,
// API calls made with these options will not be made and will immediately return
// a client-side error.
func WithContext(ctx context.Context, uid UID, data interface{}) context.Context {
	opts := *api.GetCallOptions(ctx) // make a copy
	opts.Auth = &model.AuthInfo{UID: uid, UserData: data}
	return api.WithCallOptions(ctx, &opts)
}
