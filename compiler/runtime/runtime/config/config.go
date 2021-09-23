package config

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type Access string

const (
	Public  Access = "public"
	Auth    Access = "auth"
	Private Access = "private"
)

type ServerConfig struct {
	Testing     bool
	TestService string // service being tested, if any

	Services []*Service
	// AuthData is the custom auth data type, or "" if none
	AuthData string
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
	Access  Access
	Handler func(w http.ResponseWriter, req *http.Request, ps httprouter.Params)
}
