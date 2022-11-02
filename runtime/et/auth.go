package et

import (
	"fmt"

	"encore.dev/appruntime/api"
	"encore.dev/beta/auth"
)

// OverrideUserInfo overrides the user info for the current request.
// Subsequent calls to auth.UserID and auth.Data() within the same request
// will return the given uid and data, and API calls made from the request
// will propagate the newly set user info.
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
//
// OverrideUserInfo is not safe for concurrent use with code that invokes
// auth.UserID or auth.Data() in the same request.
func OverrideUserInfo(uid auth.UID, data any) {
	Singleton.OverrideUserInfo(uid, data)
}

func (mgr *Manager) OverrideUserInfo(uid auth.UID, authData any) {
	if curr := mgr.rt.Current(); curr.Req != nil {
		authDataType := mgr.cfg.Static.AuthData
		if err := api.CheckAuthData(authDataType, uid, authData); err != nil {
			panic(fmt.Errorf("override user info: %v", err))
		}
		if rpcData := curr.Req.RPCData; rpcData != nil {
			rpcData.UserID = uid
			rpcData.AuthData = authData
		} else if testData := curr.Req.Test; testData != nil {
			testData.UserID = uid
			testData.AuthData = authData
		}
	}
}
