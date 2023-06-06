// Package auth provides the APIs to get information about the authenticated users.
//
// For more information about how authentication works with Encore applications see https://encore.dev/docs/develop/auth.
package auth

import (
	"context"

	"encore.dev/appruntime/apisdk/api"
	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/shared/reqtrack"
)

// UID is a unique identifier representing a user (a user id).
type UID = model.UID

//publicapigen:drop
type Manager struct {
	rt *reqtrack.RequestTracker
}

//publicapigen:drop
func NewManager(rt *reqtrack.RequestTracker) *Manager {
	return &Manager{rt}
}

func (mgr *Manager) UserID() (UID, bool) {
	if curr := mgr.rt.Current(); curr.Req != nil {
		if curr.Req.RPCData != nil {
			uid := curr.Req.RPCData.UserID
			return uid, uid != ""
		} else if curr.Req.Test != nil {
			uid := curr.Req.Test.UserID
			return uid, uid != ""
		}
	}
	return "", false
}

func (mgr *Manager) Data() interface{} {
	if curr := mgr.rt.Current(); curr.Req != nil {
		if curr.Req.RPCData != nil {
			return curr.Req.RPCData.AuthData
		} else if curr.Req.Test != nil {
			return curr.Req.Test.AuthData
		}
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
	// FIXME(domblack): Verify that the data is of the same type as the auth handler returns.

	opts := *api.GetCallOptions(ctx) // make a copy
	opts.Auth = &model.AuthInfo{UID: uid, UserData: data}
	return api.WithCallOptions(ctx, &opts)
}
