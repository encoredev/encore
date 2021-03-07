package config

import (
	"net/http"
)

type ServerConfig struct {
	Testing  bool
	Services []*Service
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
	Handler func(w http.ResponseWriter, req *http.Request)
}
