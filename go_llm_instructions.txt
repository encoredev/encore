## Go style guide

MUST write valid Go v1.22+ code, best practices.

## Generated folders — do not edit

`encore.gen/` + `.encore/` CLI-regenerated; never edit.

`encore.app` at repo root = app manifest — CUE-formatted **text** file, not binary, despite `.app` extension. Read like any source file.

## Encore local MCP server

Local Encore MCP wired via `.mcp.json` (registered **`encore-local`**, tool prefix `mcp__encore-local__`). Prefer for live app introspection over `Read`/grep; use docs tools when unsure of Encore API surface.

### Tools

20 tools, all prefixed `mcp__encore-local__`. MCP client gets full catalog (names, descriptions, input schemas) via `tools/list` on session init — no need enumerate here.

### Important tools (runtime-only — no filesystem equivalent)

Do what `Glob`/`Grep`/`Read` can't. For static config (services, endpoints, topics, schemas), see "When NOT to use it" below.

- **`call_endpoint`** — args: `service`, `endpoint`, `method`, `path`, `payload` (JSON string with body/query/headers/path-params), optional `auth_token` / `auth_payload` / `correlation_id`. Optional `retry_until: { predicate, timeout_ms?, interval_ms?, fail_on_timeout? }` polls **server-side** until predicate matches — predicate = `status: 200` OR `body_path: { path: ".events.0.orderID", equals: 7 }` OR `body_jq: ".events | length > 0"` (minimal subset: `<path> | length <op> N` or `<path>` truthy). Use instead of agent-side polling loops for eventually-consistent reads. **auto-starts app** if not running. Use instead of `curl` to test endpoints.
- **`wait_for_subscription_message`** — args: `topic`, optional `subscription`, `timeout_ms` (default 10000), `since` (ISO/RFC3339), `match` (top-level key/value JSON filter, e.g. `{"CustomerID":"cust_42"}`). Blocks until the next message on the topic is fully processed by a subscription handler (returns or errors), then returns `{outcome, payload, duration_ms, handler_error, trace_id}`. Use to bridge async Pub/Sub work into a synchronous verify step — beats polling read-side endpoints + introspecting via `get_pubsub`/`get_traces`.
- **`query_database`** — args: `queries` (array of `{database, query}`). Runs SQL against named DBs, multiple queries one call. Beats `encore db conn-uri` + `psql` round-trips.
- **`get_traces`** — args: optional `service`, `endpoint`, `error` (`"true"`/`"false"`), `start_time` / `end_time` (ISO), `limit`. Returns recent request traces (timing, status, ids).
- **`get_trace_spans`** — args: `trace_ids` (array IDs from `get_traces`). Returns full per-span details for deep debugging.
- **`get_objects`** — args: `buckets` (array of bucket names). Lists objects + metadata in storage buckets.
- **`search_docs`** — args: `query`, optional `hits_per_page`, `page`, `facet_filters`. Algolia-backed Encore docs search.
- **`get_docs`** — args: `paths` (array of doc paths, e.g. `/docs/go/primitives/databases`). Fetches full doc pages found via `search_docs`.

### When to reach for it

| Situation | Tool |
|---|---|
| "What services / endpoints / databases exist?" | `get_metadata` once |
| "Test my new POST /orders endpoint" | `call_endpoint` (auto-starts app) |
| "Did the Pub/Sub handler run after I published?" | `wait_for_subscription_message` |
| "Endpoint output is eventually consistent (e.g. read-after-publish)" | `call_endpoint` with `retry_until` |
| "Why did last request fail?" | `get_traces` then `get_trace_spans` |
| "How does Encore API surface work?" | `search_docs` then `get_docs` |

- **MCP only for runtime data** (`call_endpoint`, `query_database`, `get_traces`, `get_objects`, `search_docs`/`get_docs`). For **static structure** declared in source (services, endpoints, topics, subscriptions, schemas, secrets, middleware, cron jobs), `Glob`+`Grep`+`Read` faster + more reliable than `get_*` tool.

### Cloud MCP (deployed environments)

If deployed Encore Cloud, also register `encore-cloud` via `claude mcp add --transport http encore-cloud https://api.encore.cloud/mcp` for prod traces / deploy state — `encore-local` only sees local app.

## Encore check (verify app builds + endpoints work)

`encore check` compiles, boots, health-checks app, optionally runs `curl` once healthy. Use instead of manual `encore run + healthz poll + curl`.

```bash
encore check                                          # compile + boot only
encore check 'curl /ping'                             # GET a relative path
encore check 'curl /orders -X POST -d "{\"customer_id\":\"c1\",\"amount_cents\":1999}"'   # POST with JSON body (flags after path)
encore check 'curl /a; curl /b'                       # chain in one quoted string
encore check 'curl /a' 'curl /b'                      # or as separate quoted args
encore check < commands.txt                           # one curl per line, piped from file
```

Rules embedded curl DSL:
- **Paths relative** (start with `/`). Use `curl /orders/1`, **not** `curl http://localhost:4000/orders/1` — host/port supplied by `encore check`. Absolute URL fails with `curl path "http..." must be relative`.
- **Path first, flags after.** `curl /orders -X POST -d '...'` works; `curl -X POST /orders ...` fails — parser reads first token after `curl` as path, rejects flags there (`curl path "-X" must be relative`).
- One curl per quoted command. Chain `;` inside one quoted string, pass multiple quoted args, or pipe file one curl per line.

## Pub/sub verification

- Delivery async + at-least-once + not order-preserving.
- For "most-recent first" by publish order, DO NOT sort by `created_at = NOW()` or subscription-side row id — both subscription-insert order, at-least-once can flip. Sort by column monotonic with publish (entity id from event payload, or publish-time sequence in event).
- Handler-only checks, skip infra: `et.Topic(T).PublishedMessages()`.

## Encore Go domain knowledge

### Application structure

Encore uses monorepo design — one app = entire backend. Enables distributed tracing + Encore Flow via unified application model. Supports monolith + microservices with monolith-style DX.

Directory structure:
/app-name
  encore.app
  service1/
    migrations/
      1_create_table.up.sql
    service1.go
    service1_test.go
  service2/
    service2.go

Sub-packages internal to services, cannot define APIs, used for helpers + code organization.

Large apps — group related services into system directories (logical groupings, no special runtime behavior):
/app-name
  encore.app
  system1/
    service1/
    service2/
  system2/
    service3/

### API definition

Create type-safe APIs from regular Go functions via //encore:api annotation.

Access controls:
- public: Accessible to anyone on internet
- private: Only accessible within app + via cron jobs
- auth: Public but requires valid auth

Function signatures:
func Foo(ctx context.Context, p *Params) (*Response, error)  // full
func Foo(ctx context.Context) (*Response, error)             // response only
func Foo(ctx context.Context, p *Params) error               // request only
func Foo(ctx context.Context) error                          // minimal

Request/response data locations:
- header: Use `header` tag for HTTP headers
- query: Default GET/HEAD/DELETE, uses snake_case, supports basic types/slices
- body: Default other methods, uses `json` tag, supports complex types

Path parameters: Use :name for variables, *name for wildcards. Place at end of path.

Sensitive data:
- Field level: `encore:"sensitive"` tag, auto-redacted in tracing
- Endpoint level: Add `sensitive` to //encore:api annotation

Type support by location:
- headers/path: bool, numeric, string, time.Time, UUID, json.RawMessage
- query: All above plus lists
- body: All types including structs, maps, pointers

### Services

Service defined by creating at least one API within Go package. Package name = service name.

//encore:service annotation enables custom init + graceful shutdown:

type Service struct {
    // Dependencies here
}

func initService() (*Service, error) {
    // Initialization code
}

//encore:api public
func (s *Service) MyAPI(ctx context.Context) error {
    // API implementation
}

Graceful shutdown via Shutdown method:
func (s *Service) Shutdown(force context.Context)
- Graceful phase: Several seconds for completion
- Forced phase: When force context canceled, terminate immediately

### Raw endpoints

Lower-level HTTP access (webhooks, WebSockets):

//encore:api public raw
func Webhook(w http.ResponseWriter, req *http.Request) {
    // Process raw HTTP request
}

//encore:api public raw method=POST path=/webhook/:id
func Webhook(w http.ResponseWriter, req *http.Request) {
    id := encore.CurrentRequest().PathParams.Get("id")
}

### SQL databases

Encore treats SQL databases as logical resources with native PostgreSQL support.

Create database:
var tododb = sqldb.NewDatabase("todo", sqldb.DatabaseConfig{
    Migrations: "./migrations",
})

Migration naming: number_description.up.sql (e.g., 1_create_table.up.sql)
Migrations folder structure:
service/
  migrations/
    1_create_table.up.sql
    2_add_field.up.sql
  service.go

Data operations:
// Insert
_, err := tododb.Exec(ctx, `
    INSERT INTO todo_item (id, title, done)
    VALUES ($1, $2, $3)
`, id, title, done)

// Query
err := tododb.QueryRow(ctx, `
    SELECT id, title, done FROM todo_item LIMIT 1
`).Scan(&item.ID, &item.Title, &item.Done)
// Use errors.Is(err, sqldb.ErrNoRows) for no results

Always Scan into typed struct fields, never `interface{}` slots — compiler enforces column-to-field shape + catches drift between SELECT list + destination struct.

CLI commands:
- encore db shell database-name [--env=name] - Open psql shell
- encore db conn-uri database-name [--env=name] - Output connection string
- encore db proxy [--env=name] - Setup local connection proxy

### External databases

Existing databases — create dedicated package with lazy connection pool:

package externaldb

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v4/pgxpool"
    "go4.org/syncutil"
)

func Get(ctx context.Context) (*pgxpool.Pool, error) {
    err := once.Do(func() error {
        var err error
        pool, err = setup(ctx)
        return err
    })
    return pool, err
}

var (
    once syncutil.Once
    pool *pgxpool.Pool
)

var secrets struct {
    ExternalDBPassword string
}

func setup(ctx context.Context) (*pgxpool.Pool, error) {
    connString := fmt.Sprintf("postgresql://%s:%s@hostname:port/dbname?sslmode=require",
        "user", secrets.ExternalDBPassword)
    return pgxpool.Connect(ctx, connString)
}

Works with Cassandra, DynamoDB, BigTable, MongoDB, Neo4j, other services.

### Shared databases

Default: per-service databases for isolation. Share via sqldb.Named:

// In report service, access todo service's database:
var todoDB = sqldb.Named("todo")

//encore:api method=GET path=/report/todo
func CountCompletedTodos(ctx context.Context) (*ReportResponse, error) {
    var report ReportResponse
    err := todoDB.QueryRow(ctx,`
        SELECT COUNT(*) FROM todo_item WHERE completed = TRUE
    `).Scan(&report.Total)
    return &report, err
}

### Cron jobs

Declarative periodic tasks. Does not run locally or in Preview Environments.

import "encore.dev/cron"

var _ = cron.NewJob("welcome-email", cron.JobConfig{
    Title:    "Send welcome emails",
    Every:    2 * cron.Hour,
    Endpoint: SendWelcomeEmail,
})

//encore:api private
func SendWelcomeEmail(ctx context.Context) error {
    return nil
}

Scheduling options:
- Every: Must divide 24 hours evenly (e.g., 10 * cron.Minute, 6 * cron.Hour)
- Schedule: Cron expressions (e.g., "0 4 15 * *" for 4am UTC on 15th)

Requirements: Endpoints must be idempotent, no request parameters, signature func(context.Context) error or func(context.Context) (*T, error)

### Caching

Redis-based distributed caching.

import "encore.dev/storage/cache"

var MyCacheCluster = cache.NewCluster("my-cache-cluster", cache.ClusterConfig{
    EvictionPolicy: cache.AllKeysLRU,
})

// Keyspace with type safety
var RequestsPerUser = cache.NewIntKeyspace[auth.UID](cluster, cache.KeyspaceConfig{
    KeyPattern:    "requests/:key",
    DefaultExpiry: cache.ExpireIn(10 * time.Second),
})

// Structured keys
type MyKey struct {
    UserID auth.UID
    ResourcePath string
}
var ResourceRequestsPerUser = cache.NewIntKeyspace[MyKey](cluster, cache.KeyspaceConfig{
    KeyPattern:    "requests/:UserID/:ResourcePath",
    DefaultExpiry: cache.ExpireIn(10 * time.Second),
})

Supports strings, integers, floats, structs, sets, ordered lists.

### Object storage

Cloud-agnostic API compatible with S3, GCS, S3-compatible services.

var ProfilePictures = objects.NewBucket("profile-pictures", objects.BucketConfig{
    Versioned: false,
})

// Public bucket with CDN
var PublicAssets = objects.NewBucket("public-assets", objects.BucketConfig{
    Public: true,
})

Operations: Upload, Download, List, Remove, Attrs, Exists

Bucket references for permissions:
type myPerms interface {
    objects.Downloader
    objects.Uploader
}
ref := objects.BucketRef[myPerms](bucket)

### Pub/Sub

Async event broadcasting with automatic infra provisioning.

type SignupEvent struct{ UserID int }

var Signups = pubsub.NewTopic[*SignupEvent]("signups", pubsub.TopicConfig{
    DeliveryGuarantee: pubsub.AtLeastOnce,
})

// Publishing
messageID, err := Signups.Publish(ctx, &SignupEvent{UserID: id})

// Topic reference
signupRef := pubsub.TopicRef[pubsub.Publisher[*SignupEvent]](Signups)

// Subscribing
var _ = pubsub.NewSubscription(
    user.Signups, "send-welcome-email",
    pubsub.SubscriptionConfig[*SignupEvent]{
        Handler: SendWelcomeEmail,
    },
)

// Method handler with dependency injection
var _ = pubsub.NewSubscription(
    user.Signups, "send-welcome-email",
    pubsub.SubscriptionConfig[*SignupEvent]{
        Handler: pubsub.MethodHandler((*Service).SendWelcomeEmail),
    },
)

Delivery guarantees:
- AtLeastOnce: Handlers must be idempotent
- ExactlyOnce: Stronger guarantees (AWS: 300 msg/sec, GCP: 3000+ msg/sec)

Ordering: Use OrderingAttribute matching pubsub-attr tag

Testing:
msgs := et.Topic(Signups).PublishedMessages()
assert.Len(t, msgs, 1)

### Secrets

Built-in secrets manager for API keys, passwords, private keys.

var secrets struct {
    SSHPrivateKey string
    GitHubAPIToken string
}

func callGitHub(ctx context.Context) {
    req.Header.Add("Authorization", "token " + secrets.GitHubAPIToken)
}

CLI management:
- encore secret set --type production secret-name
- encore secret set --type development secret-name
- encore secret set --env env-name secret-name (env-specific override)

Types: production (prod), development (dev), preview (pr), local

Local override via .secrets.local.cue:
GitHubAPIToken: "my-local-override-token"

### API calls

Call APIs like regular functions with automatic type checking:

import "encore.app/hello"

//encore:api public
func MyOtherAPI(ctx context.Context) error {
    resp, err := hello.Ping(ctx, &hello.PingParams{Name: "World"})
    if err == nil {
        log.Println(resp.Message) // "Hello, World!"
    }
    return err
}

### Errors

Structured errors via encore.dev/beta/errs package.

return &errs.Error{
    Code: errs.NotFound,
    Message: "sprocket not found",
}
// Returns HTTP 404 {"code": "not_found", "message": "sprocket not found"}

Wrapping:
errs.Wrap(err, msg, metaPairs...)
errs.WrapCode(err, code, msg, metaPairs...)

Builder pattern:
eb := errs.B().Meta("board_id", params.ID)
return eb.Code(errs.NotFound).Msg("board not found").Err()

Error codes: OK(200), Canceled(499), Unknown(500), InvalidArgument(400), DeadlineExceeded(504), NotFound(404), AlreadyExists(409), PermissionDenied(403), ResourceExhausted(429), FailedPrecondition(400), Aborted(409), OutOfRange(400), Unimplemented(501), Internal(500), Unavailable(503), DataLoss(500), Unauthenticated(401)

Inspection: errs.Code(err), errs.Meta(err), errs.Details(err)

### Authentication

Flexible auth with different access levels.

import "encore.dev/beta/auth"

// Basic
//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, error) {
    // Validate token and return user ID
}

// With user data
type Data struct {
    Username string
}

//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, *Data, error) {
    // Return user ID and custom data
}

// Structured auth params
type MyAuthParams struct {
    SessionCookie *http.Cookie `cookie:"session"`
    ClientID string `query:"client_id"`
    Authorization string `header:"Authorization"`
}

//encore:authhandler
func AuthHandler(ctx context.Context, p *MyAuthParams) (auth.UID, error) {
    // Process structured auth params
}

Usage: auth.Data(), auth.UserID()
Override for testing: auth.WithContext(ctx, auth.UID("my-user-id"), &MyAuthData{})

Error handling:
return "", &errs.Error{
    Code: errs.Unauthenticated,
    Message: "invalid token",
}

### Configuration

Env-specific config via CUE files.

package mysvc

import "encore.dev/config"

type SomeConfigType struct {
    ReadOnly config.Bool
    Example  config.String
}

var cfg *SomeConfigType = config.Load[*SomeConfigType]()

CUE tags for constraints:
type FooBar {
    A int `cue:">100"`
    B int `cue:"A-50"`
    C int `cue:"A+B"`
}

Config types: config.String, config.Bool, config.Int, config.Float64, config.Time, config.UUID, config.Value[T], config.Values[T]

Meta values:
- APIBaseURL, Environment.Name, Environment.Type (production/development/ephemeral/test), Environment.Cloud (aws/gcp/encore/local)

Testing: et.SetCfg(cfg.SendEmails, true)

CUE patterns:
- Defaults: value: type | *default_value
- Switch: array with conditionals, take [0]

### CORS

Configure in encore.app file:
- debug: Enable CORS debug logging
- allow_headers: Additional accepted headers ("*" allows all)
- expose_headers: Additional exposed headers
- allow_origins_without_credentials: Defaults to ["*"]
- allow_origins_with_credentials: For authenticated requests, supports wildcards like "https://*.example.com"

### Metadata

Access app + request info via encore.dev package.

// Application metadata
meta := encore.Meta()
// meta.AppID, meta.APIBaseURL, meta.Environment, meta.Build, meta.Deploy

// Request metadata
req := encore.CurrentRequest()
// req.Service, req.Endpoint, req.Path, req.StartTime

// Cloud-specific behavior
switch encore.Meta().Environment.Cloud {
case encore.CloudAWS:
    return writeIntoRedshift(ctx, action, user)
case encore.CloudGCP:
    return writeIntoBigQuery(ctx, action, user)
}

### Middleware

Reusable code running before/after API requests.

//encore:middleware global target=all
func ValidationMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
    payload := req.Data().Payload
    if validator, ok := payload.(interface { Validate() error }); ok {
        if err := validator.Validate(); err != nil {
            err = errs.WrapCode(err, errs.InvalidArgument, "validation failed")
            return middleware.Response{Err: err}
        }
    }
    return next(req)
}

// With dependency injection
//encore:middleware target=all
func (s *Service) MyMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
    // Implementation
}

// Tag-based targeting
//encore:middleware target=tag:cache
func CachingMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
    // ...
}

//encore:api public method=GET path=/user/:id tag:cache
func GetUser(ctx context.Context, id string) (*User, error) {
    // Implementation
}

Ordering: Global before service-specific, lexicographic by filename.

### Mocking

Built-in mocking for isolated testing.

// Mock endpoint for single test
func Test_Something(t *testing.T) {
    t.Parallel()
    et.MockEndpoint(products.GetPrice, func(ctx context.Context, p *products.PriceParams) (*products.PriceResponse, error) {
        return &products.PriceResponse{Price: 100}, nil
    })
}

// Mock endpoint for all tests in package
func TestMain(m *testing.M) {
    et.MockEndpoint(products.GetPrice, func(ctx context.Context, p *products.PriceParams) (*products.PriceResponse, error) {
        return &products.PriceResponse{Price: 100}, nil
    })
    os.Exit(m.Run())
}

// Mock entire service
et.MockService("products", &products.Service{
    SomeField: "a testing value",
})

// Type-safe service mocking
et.MockService[products.Interface]("products", &myMockObject{})

### Testing

Run tests: encore test ./...
Supports all standard go test flags. Built-in tracing at localhost:9400.

Database testing:
- Automatic setup in separate cluster, optimized for speed
- Temporary databases: et.NewTestDatabase() creates isolated, fully migrated DB

Service structs: Lazy init, instance sharing between tests
- Isolate: et.EnableServiceInstanceIsolation()

### Validation

Automatic request validation via Validate() method.

type MyRequest struct {
    Email string
}

func (r *MyRequest) Validate() error {
    if !isValidEmail(r.Email) {
        return &errs.Error{Code: errs.InvalidArgument, Message: "invalid email"}
    }
    return nil
}

Validation runs after deserialization, before handler. Non-errs.Error errors become InvalidArgument (HTTP 400).

### CGO

Enable in encore.app:
{
  "id": "my-app-id",
  "build": {
    "cgo_enabled": true
  }
}
Uses Ubuntu builder with gcc. Libraries must support static linking.

### Clerk auth

Implement Clerk auth:

package auth

import "github.com/clerkinc/clerk-sdk-go/clerk"

type Service struct {
    client clerk.Client
}

func initService() (*Service, error) {
    client, err := clerk.NewClient(secrets.ClientSecretKey)
    if err != nil {
        return nil, err
    }
    return &Service{client: client}, nil
}

type UserData struct {
    ID                    string
    Username              *string
    FirstName             *string
    LastName              *string
    ProfileImageURL       string
    PrimaryEmailAddressID *string
    EmailAddresses        []clerk.EmailAddress
}

//encore:authhandler
func (s *Service) AuthHandler(ctx context.Context, token string) (auth.UID, *UserData, error) {
    // Token verification and user data retrieval
}

Set secrets:
- encore secret set --prod ClientSecretKey
- encore secret set --dev ClientSecretKey

### Dependency injection

Add dependencies as struct fields for easy testing:

package email

//encore:service
type Service struct {
    sendgridClient *sendgrid.Client
}

func initService() (*Service, error) {
    client, err := sendgrid.NewClient()
    if err != nil {
        return nil, err
    }
    return &Service{sendgridClient: client}, nil
}

//encore:api private
func (s *Service) Send(ctx context.Context, p *SendParams) error {
    // Use s.sendgridClient
}

// For testing, use interface
type sendgridClient interface {
    SendEmail(...)
}

func TestFoo(t *testing.T) {
    svc := &Service{sendgridClient: &myMockClient{}}
    // Test
}

### Pub/Sub outbox

Transactional outbox pattern for database + Pub/Sub consistency.

var SignupsTopic = pubsub.NewTopic[*SignupEvent](/* ... */)
ref := pubsub.TopicRef[pubsub.Publisher[*SignupEvent]](SignupsTopic)
ref = outbox.Bind(ref, outbox.TxPersister(tx))

Required schema:
CREATE TABLE outbox (
    id BIGSERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    data JSONB NOT NULL,
    inserted_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX outbox_topic_idx ON outbox (topic, id);

Relay setup:
type Service struct {
    signupsRef pubsub.Publisher[*SignupEvent]
}

func initService() (*Service, error) {
    relay := outbox.NewRelay(outbox.SQLDBStore(db))
    signupsRef := pubsub.TopicRef[pubsub.Publisher[*SignupEvent]](SignupsTopic)
    outbox.RegisterTopic(relay, signupsRef)
    go relay.PollForMessage(context.Background(), -1)
    return &Service{signupsRef: signupsRef}, nil
}

Supports: encore.dev/storage/sqldb, database/sql, github.com/jackc/pgx/v5

### Encore Toolbar (frontend dev panel)

Dev-only browser panel: intercepts frontend HTTP calls, captures Encore trace IDs, jumps to traces in the dashboard. Install: add `<script src="https://encore.dev/encore-toolbar.js"></script>` early in `<head>` (no `async`/`defer`). Reach for it when a frontend talks to an Encore Go backend.

### Example apps

- Hello World: https://github.com/encoredev/examples/tree/main/hello-world
- URL Shortener: https://github.com/encoredev/examples/tree/main/url-shortener
- Uptime Monitor: https://github.com/encoredev/examples/tree/main/uptime

## Encore CLI reference

Execution:
- encore help [command] - Lists available commands (or shows usage for specific command). Reach for it when unsure which subcommand or flag to use.
- encore run [--debug] [--watch=true] [-p port] [flags] - Runs app. Before starting yourself, check if already running by polling `http://localhost:<port>/__encore/healthz` (default port `4000`) — `200` = healthy, hit endpoints directly without re-running.
- encore check ['curl ...'; 'curl ...'] | encore check < commands.txt - Compile + boot app, verify every service healthy, optionally run curl commands. See dedicated section above.
- encore test ./... [go test flags] - Wraps `go test ./...` with Encore's infra provisioning — boots isolated test databases, Pub/Sub, etc. before handing off to `go test`. Accepts all `go test` flags, plus `--prepare`, `--codegen-debug`, `--trace`.
- encore exec path/to/script [args...] - Run an executable script against the local Encore app

App management:
- encore app clone [app-id] [directory] - Clone app
- encore app create [name] [--example=name] [-l go] - Create new app
- encore app init [name] - Create from existing repo
- encore app link [app-id] [-f] - Link app with server

Authentication:
- encore auth login/logout/signup/whoami

Daemon:
- encore daemon - Restart daemon
- encore daemon env - Output environment info

Database:
- encore db shell database-name [--env=name] - psql shell (--write, --admin, --superuser)
- encore db conn-uri database-name [--env=name] - Connection string
- encore db proxy [--env=name] - Local proxy
- encore db reset <db-names...|--all> - Reset databases

Code generation:
- encore gen client [app-id] [--env=name] [--lang=lang] - Generate API client
  Languages: go, typescript, javascript, openapi

Logging:
- encore logs [--env=prod] [--json] [-q] - Stream logs

Kubernetes:
- encore k8s configure --env=ENV_NAME - Update kubectl config

Secrets:
- encore secret set --type TYPE <secret-name> (types: production, development, preview, local)
- encore secret set --env env-name <secret-name>
- encore secret list [keys...]
- encore secret delete <id>

Namespaces (infrastructure environments, alias `encore ns`):
- encore namespace list [--output=columns|json] - List infra namespaces
- encore namespace create NAME - Create a namespace
- encore namespace switch [--create] NAME - Switch active namespace
- encore namespace delete NAME - Delete a namespace

MCP (programmatic access to the local app — same surface as `.mcp.json`):
- encore mcp run - stdio-based MCP session
- encore mcp start - SSE-based MCP session; prints the SSE URL

Config:
- encore config <key> [<value>] [--app|--global|--all] - Get/set CLI configuration

Telemetry:
- encore telemetry - Status
- encore telemetry enable / disable - Toggle telemetry reporting

LLM rules:
- encore llm-rules init - Generate LLM rule files for this project

Random data (utility):
- encore rand uuid [-1|-4|-6|-7] - Generate UUID (default v4)
- encore rand bytes N [-f format] - Generate N random bytes
- encore rand words [--sep=SEP] NUM - Generate memorable passphrase

Deploy (alpha):
- encore alpha deploy --env=<name> (--commit=<sha> | --branch=<name>) - Deploy app to a cloud env

Version:
- encore version - Report version
- encore version update - Check and apply updates

Build:
- encore build docker IMAGE_TAG [--base string] [--push] [--cgo] [--os] [--arch] - Build Docker image