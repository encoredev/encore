package config

import (
	"net/http"
	"reflect"

	"github.com/julienschmidt/httprouter"
)

type ServerConfig struct {
	Testing  bool
	Services []*Service
	// AuthData is the custom auth data type, or nil
	AuthData reflect.Type
}

type Service struct {
	Name      string
	RelPath   string // relative path to service pkg (from app root)
	Endpoints []*Endpoint
	SQLDB     bool // does the service use sqldb?
}

type Endpoint struct {
	Name    string
	Raw     bool
	Path    string
	Methods []string
	Handler func(w http.ResponseWriter, req *http.Request, ps httprouter.Params)
}
