package et

import (
	"fmt"

	"encore.dev/appruntime/apisdk/api"
	"encore.dev/beta/auth"
)

func (mgr *Manager) OverrideAuthInfo(uid auth.UID, authData any) {
	if curr := mgr.rt.Current(); curr.Req != nil {
		if err := api.CheckAuthData(uid, authData); err != nil {
			panic(fmt.Errorf("override auth info: %v", err))
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
