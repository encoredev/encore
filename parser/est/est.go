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
	"encr.dev/parser/selector"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

type Application struct {
	ModulePath    string
	Packages      []*Package
	Services      []*Service
	CronJobs      []*CronJob
	PubSubTopics  []*PubSubTopic
	CacheClusters []*CacheCluster
	Decls         []*schema.Decl
	AuthHandler   *AuthHandler
	Middleware    []*Middleware
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
	Name       string
	Root       *Package
	Pkgs       []*Package
	RPCs       []*RPC
	Middleware []*Middleware

	// Struct is the dependency injection struct, or nil if none exists.
	Struct *ServiceStruct
}

// ServiceStruct describes a dependency injection struct a particular service defines.
type ServiceStruct struct {
	Name string
	Svc  *Service
	File *File // where the struct is defined
	Doc  string
	Decl *ast.TypeSpec
	RPCs []*RPC // RPCs defined on the service struct

	// Init is the function for initializing this group.
	// It is nil if there is no initialization function.
	Init     *ast.FuncDecl
	InitFile *File // where the init func is declared
}

type CronJob struct {
	ID       string
	Title    string
	Doc      string
	Schedule string
	RPC      *RPC
	DeclFile *File
	AST      *ast.Ident
}

func (cj *CronJob) Type() ResourceType         { return CronJobResource }
func (cj *CronJob) File() *File                { return cj.DeclFile }
func (cj *CronJob) Ident() *ast.Ident          { return cj.AST }
func (cj *CronJob) NodeType() NodeType         { return CronJobNode }
func (cj *CronJob) AllowOnlyParsedUsage() bool { return true }

func (cj *CronJob) IsValid() (bool, error) {
	switch {
	case cj.ID == "":
		return false, errors.New("field ID is required")
	case cj.Title == "":
		return false, errors.New("field Title is required")
	case cj.RPC == nil:
		return false, errors.New("field RPC is required")
	case cj.Schedule == "":
		return false, errors.New("field Schedule is required")
	}

	return true, nil
}

type PubSubTopic struct {
	Name              string          // The unique name of the pub sub topic
	Doc               string          // The documentation on the pub sub topic
	DeliveryGuarantee PubSubGuarantee // What guarantees does the pub sub topic have?
	OrderingKey       string          // What field in the message type should be used to ensure First-In-First-Out (FIFO) for messages with the same key
	DeclFile          *File           // What file the topic is declared in
	MessageType       *Param          // The message type of the pub sub topic
	IdentAST          *ast.Ident      // The AST node representing the value this topic is bound against

	Subscribers []*PubSubSubscriber
	Publishers  []*PubSubPublisher
}

func (p *PubSubTopic) Type() ResourceType         { return PubSubTopicResource }
func (p *PubSubTopic) File() *File                { return p.DeclFile }
func (p *PubSubTopic) Ident() *ast.Ident          { return p.IdentAST }
func (p *PubSubTopic) NodeType() NodeType         { return PubSubTopicDefNode }
func (p *PubSubTopic) AllowOnlyParsedUsage() bool { return false }

type PubSubGuarantee int

const (
	AtLeastOnce PubSubGuarantee = iota
	ExactlyOnce
)

type PubSubSubscriber struct {
	Name     string       // The unique name of the subscriber
	Topic    *PubSubTopic // The topic the subscriber is registered against
	CallSite ast.Node     // The AST node representing the creation of the subscriber
	Func     ast.Node     // The function that is the subscriber (either a *ast.FuncLit or a *ast.FuncDecl)
	FuncFile *File        // The file the subscriber function is declared in
	DeclFile *File        // The file that the subscriber is defined in
	IdentAST *ast.Ident   // The AST node representing the value this topic is bound against

	AckDeadline      time.Duration
	MessageRetention time.Duration
	MinRetryBackoff  time.Duration
	MaxRetryBackoff  time.Duration
	MaxRetries       int64 // number of attempts
}

func (p *PubSubSubscriber) Type() ResourceType         { return PubSubTopicResource }
func (p *PubSubSubscriber) File() *File                { return p.DeclFile }
func (p *PubSubSubscriber) Ident() *ast.Ident          { return p.IdentAST }
func (p *PubSubSubscriber) NodeType() NodeType         { return PubSubSubscriberNode }
func (p *PubSubSubscriber) AllowOnlyParsedUsage() bool { return true }

type PubSubPublisher struct {
	DeclFile *File // The file the publisher is declared in
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
	Tags        selector.Set

	// SvcStruct is the service struct this RPC is defined on,
	// or nil otherwise. It is always a pointer receiver.
	SvcStruct *ServiceStruct
}

type NodeType int

const (
	RPCDefNode NodeType = iota + 1
	RPCRefNode
	SQLDBNode
	RLogNode
	SecretsNode
	CronJobNode
	PubSubTopicDefNode
	PubSubPublisherNode
	PubSubSubscriberNode
	CacheClusterDefNode
	CacheKeyspaceDefNode
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
	Svc    *Service
	Name   string
	Doc    string
	Func   *ast.FuncDecl
	File   *File
	Params *schema.Type // builtin string or named type

	// AuthData is the custom auth data type the app may specify
	// as part of the returns from the auth handler.
	// It is nil if no such auth data type is specified.
	AuthData *Param
}

type Middleware struct {
	Name   string
	Doc    string
	Global bool
	Target selector.Set

	Func *ast.FuncDecl
	File *File

	Pkg       *Package       // pkg this middleware is defined in
	Svc       *Service       // nil if global
	SvcStruct *ServiceStruct // nil if not defined on a service struct
}

type Resource interface {
	Type() ResourceType
	File() *File
	Ident() *ast.Ident
	NodeType() NodeType
	AllowOnlyParsedUsage() bool // If true this resource can only be used with registered resource parsers. If false we allow any usage.
}

//go:generate stringer -type=ResourceType

type ResourceType int

const (
	SQLDBResource ResourceType = iota + 1
	CronJobResource
	PubSubTopicResource
	PubSubSubscriptionResource
	CacheClusterResource
	CacheKeyspaceResource
)

type SQLDB struct {
	DeclFile *File
	DeclName *ast.Ident // where the resource is declared
	DBName   string
}

func (r *SQLDB) Type() ResourceType         { return SQLDBResource }
func (r *SQLDB) File() *File                { return r.DeclFile }
func (r *SQLDB) Ident() *ast.Ident          { return r.DeclName }
func (r *SQLDB) NodeType() NodeType         { return SQLDBNode }
func (r *SQLDB) AllowOnlyParsedUsage() bool { return false }

type CacheCluster struct {
	Name           string     // The unique name of the cache cluster
	Doc            string     // The documentation on the cluster
	DeclFile       *File      // What file the cache is declared in
	IdentAST       *ast.Ident // The AST node representing the value this cache cluster is bound against
	EvictionPolicy string

	Keyspaces []*CacheKeyspace
}

func (p *CacheCluster) Type() ResourceType         { return CacheClusterResource }
func (p *CacheCluster) File() *File                { return p.DeclFile }
func (p *CacheCluster) Ident() *ast.Ident          { return p.IdentAST }
func (p *CacheCluster) NodeType() NodeType         { return CacheClusterDefNode }
func (p *CacheCluster) AllowOnlyParsedUsage() bool { return false }

type CacheKeyspace struct {
	Cluster   *CacheCluster
	Svc       *Service
	Doc       string     // The documentation on the cluster
	DeclFile  *File      // What file the cache is declared in
	IdentAST  *ast.Ident // The AST node representing the value this cache cluster is bound against
	ConfigLit *ast.CompositeLit

	KeyType   *schema.Type // The key type for this keyspace
	ValueType *schema.Type // The value type for this keyspace
	Path      *paths.Path  // The keyspace path
}

func (p *CacheKeyspace) Type() ResourceType         { return CacheKeyspaceResource }
func (p *CacheKeyspace) File() *File                { return p.DeclFile }
func (p *CacheKeyspace) Ident() *ast.Ident          { return p.IdentAST }
func (p *CacheKeyspace) NodeType() NodeType         { return CacheKeyspaceDefNode }
func (p *CacheKeyspace) AllowOnlyParsedUsage() bool { return false }
