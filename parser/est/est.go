// Package est provides the Encore Syntax Tree (EST).
//
// It is an Encore-specific syntax tree that represents the higher-level representation
// of the application that Encore understands.
package est

import (
	"errors"
	"go/ast"
	"go/token"
	"time"

	"encr.dev/parser/paths"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

type Application struct {
	ModulePath  string
	Packages    []*Package
	Services    []*Service
	CronJobs    []*CronJob
	Decls       []*schema.Decl
	AuthHandler *AuthHandler
}

type File struct {
	Name       string   // file name ("foo.go")
	Pkg        *Package // package it belongs to
	Path       string   // filesystem path
	AST        *ast.File
	Token      *token.File
	Contents   []byte
	References map[ast.Node]*Node
}

type Package struct {
	AST        *ast.Package
	Name       string
	Doc        string
	ImportPath string // import path
	RelPath    string // import path relative to app root
	Dir        string // filesystem path
	Files      []*File
	Service    *Service // the service this package belongs to, if any
	Secrets    []string
	Resources  []Resource
}

// A Service is a Go package that defines one or more RPCs.
// Its name is defined by the Go package name.
// A Service may not be a located in a child directory of another service.
type Service struct {
	Name string
	Root *Package
	Pkgs []*Package
	RPCs []*RPC
}

type CronJob struct {
	ID       string
	Name     string
	Doc      string
	Schedule string
	Every    time.Duration
	RPC      *RPC
	AST      *ast.ValueSpec
}

func (cj *CronJob) IsValid() (bool, error) {
	switch {
	case cj.ID == "":
		return false, errors.New("field ID is required")
	case cj.Name == "":
		return false, errors.New("field Name is required")
	case cj.RPC == nil:
		return false, errors.New("field RPC is required")
	case cj.Schedule == "" && cj.Every == 0:
		return false, errors.New("either Every or Schedule must be set")
	case cj.Schedule != "" && cj.Every != 0:
		return false, errors.New("cannot set both Every and Schedule")
	}

	return true, nil
}

type Param struct {
	IsPtr bool
	Type  *schema.Type
}

type AccessType string

const (
	Public  AccessType = "public"
	Private AccessType = "private"
	// Auth is like public but requires authentication.
	Auth AccessType = "auth"
)

type RPC struct {
	Svc         *Service
	Name        string
	Doc         string
	Func        *ast.FuncDecl
	File        *File
	Access      AccessType
	Raw         bool
	Path        *paths.Path
	HTTPMethods []string
	Request     *Param // request data; nil for Raw RPCs
	Response    *Param // response data; nil for Raw RPCs
}

type NodeType int

const (
	RPCDefNode NodeType = iota + 1
	RPCRefNode
	SQLDBNode
	RLogNode
	SecretsNode
)

type Node struct {
	Type NodeType
	// If Type == RPCDefNode or RPCCallNode,
	// RPC is the RPC being defined or called.
	RPC *RPC

	// If Type == SQLDBNode or RLogNode,
	// Func is the func name being called.
	Func string

	// Resource this refers to, if any
	Res Resource
}

type AuthHandler struct {
	Svc      *Service
	Name     string
	Doc      string
	Func     *ast.FuncDecl
	File     *File
	AuthData *Param // or nil
}

type Resource interface {
	Type() ResourceType
	File() *File
	Ident() *ast.Ident
}

//go:generate stringer -type=ResourceType

type ResourceType int

const (
	SQLDBResource ResourceType = iota + 1
)

type SQLDB struct {
	DeclFile *File
	DeclName *ast.Ident // where the resource is declared
	DBName   string
}

func (r *SQLDB) Type() ResourceType { return SQLDBResource }
func (r *SQLDB) File() *File        { return r.DeclFile }
func (r *SQLDB) Ident() *ast.Ident  { return r.DeclName }
