package app

import (
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/parsectx"
)

func (d *Desc) validateAuthHandlers(pc *parsectx.Context, fw *apiframework.AppDesc) {
	_, found := fw.AuthHandler.Get()
	if !found {
		return
	}

}
