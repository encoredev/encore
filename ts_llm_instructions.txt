## Node.js style guide

MUST write valid TypeScript w/ Node.js v20+ features + best practices:
- ES6+ syntax always.
- Built-in `fetch` for HTTP, not `node-fetch`.
- Node.js `import`, never `require`.
## TypeScript style guide

- interface/type for complex objects
- Built-in utility types (Record, Partial, Pick) over any

## Generated folders — do not edit

`encore.gen/` + `.encore/` = CLI-regenerated; never edit.

`encore.app` at repo root = app manifest — JSON **text** file, not binary despite `.app` ext. Read like any source file.

## Encore local MCP server

Local Encore MCP wired via `.mcp.json` (registered as **`encore-local`**, tool prefix `mcp__encore-local__`). Prefer for live app introspection over `Read`/grep; use docs tools when unsure of Encore API surface.

### Tools

20 tools, all prefixed `mcp__encore-local__`. MCP client gets full catalog (names, descriptions, schemas) via `tools/list` on session init — no list here.

### Important tools (runtime-only — no filesystem equivalent)

These do what `Glob`/`Grep`/`Read` can't. For static config (services, endpoints, topics, schemas), see "When NOT to use it" below.

- **`call_endpoint`** — args: `service`, `endpoint`, `method`, `path`, `payload` (JSON string w/ body/query/headers/path-params), optional `auth_token` / `auth_payload` / `correlation_id`. Optional `retry_until: { predicate, timeout_ms?, interval_ms?, fail_on_timeout? }` polls **server-side** until predicate matches — predicate = `status: 200` OR `body_path: { path: ".events.0.orderID", equals: 7 }` OR `body_jq: ".events | length > 0"` (minimal subset: `<path> | length <op> N` or `<path>` truthy). Use instead of agent-side polling loops for eventually-consistent reads. **auto-starts app** if not running. Use instead of `curl` to test endpoints.
- **`wait_for_subscription_message`** — args: `topic`, optional `subscription`, `timeout_ms` (default 10000), `since` (ISO/RFC3339), `match` (top-level key/value JSON filter, e.g. `{"customerID":"cust_42"}`). Blocks until the next message on the topic is fully processed by a subscription handler (returns or errors), then returns `{outcome, payload, duration_ms, handler_error, trace_id}`. Use to bridge async Pub/Sub work into a synchronous verify step — beats polling read-side endpoints + introspecting via `get_pubsub`/`get_traces`.
- **`query_database`** — args: `queries` (array of `{database, query}`). Runs SQL against named DBs, multi queries per call. Beats `encore db conn-uri` + `psql` round-trips.
- **`get_traces`** — args: optional `service`, `endpoint`, `error` (`"true"`/`"false"`), `start_time` / `end_time` (ISO), `limit`. Recent request traces (timing, status, ids).
- **`get_trace_spans`** — args: `trace_ids` (array from `get_traces`). Full per-span details for deep debug.
- **`get_objects`** — args: `buckets` (array of bucket names). Lists objects + metadata in storage buckets.
- **`search_docs`** — args: `query`, optional `hits_per_page`, `page`, `facet_filters`. Algolia-backed Encore docs search.
- **`get_docs`** — args: `paths` (array of doc paths, e.g. `/docs/ts/primitives/databases`). Fetches full doc pages found via `search_docs`.

### When to reach for it

| Situation | Tool |
|---|---|
| "What services / endpoints / databases exist?" | `get_metadata` once |
| "Test my new POST /orders endpoint" | `call_endpoint` (auto-starts app) |
| "Did the Pub/Sub handler run after I published?" | `wait_for_subscription_message` |
| "Endpoint output is eventually consistent (e.g. read-after-publish)" | `call_endpoint` w/ `retry_until` |
| "Why did last request fail?" | `get_traces` then `get_trace_spans` |
| "How does Encore API surface work?" | `search_docs` then `get_docs` |

- **MCP only for runtime data** - For **static structure** in source (services, endpoints, topics, subscriptions, schemas, secrets, middleware, cron jobs), `Glob`+`Grep`+`Read` = faster + more reliable than `get_*` tool.

### Cloud MCP (deployed environments)

If deployed to Encore Cloud, also register `encore-cloud` via `claude mcp add --transport http encore-cloud https://api.encore.cloud/mcp` for prod traces / deploy state — `encore-local` only sees local app.

## Encore check (verify app builds + endpoints work)

`encore check` compiles, boots, health-checks app, optionally runs `curl` cmds once healthy. Use instead of manual `encore run + healthz poll + curl` pattern.

```bash
encore check                                          # compile + boot only
encore check 'curl /ping'                             # GET a relative path
encore check 'curl /orders -X POST -d "{\"customer_id\":\"c1\",\"amount_cents\":1999}"'   # POST with JSON body (flags after path)
encore check 'curl /a; curl /b'                       # chain in one quoted string
encore check 'curl /a' 'curl /b'                      # or as separate quoted args
encore check < commands.txt                           # one curl per line, piped from file
```

Rules for embedded curl DSL:
- **Paths MUST be relative** (start w/ `/`). Use `curl /orders/1`, **not** `curl http://localhost:4000/orders/1` — host/port supplied by `encore check`. Absolute URL fails w/ `curl path "http..." must be relative`.
- **Path first, flags after.** `curl /orders -X POST -d '...'` works; `curl -X POST /orders ...` fails — parser reads first token after `curl` as path, rejects flags (`curl path "-X" must be relative`).
- One curl per quoted cmd. Chain w/ `;` inside one quoted string, pass multi quoted args, or pipe file w/ one curl per line.

## Pub/sub verification

- Delivery = async + at-least-once + not order-preserving.
- For "most-recent first" by publish order, do NOT sort by `created_at = NOW()` nor subscription-side row id — both = subscription-insert order, at-least-once can flip. Sort by column monotonic w/ publish (entity id from event payload, or publish-time seq in event).
- For handler-only checks, skip infra: `et.Topic(T).PublishedMessages()`.

## Encore.ts domain knowledge

### API definition

Encore.ts = type-safe TS API endpoints w/ built-in request validation. APIs = async fns w/ TS interfaces for req/resp. Source parsing enables auto req validation against schemas.

Syntax:
import { api } from "encore.dev/api";
export const endpoint = api(options, async handler);

Options: method (HTTP method), expose (bool public access, default: false), auth (bool requires auth), path (URL path pattern)

Example:
import { api } from "encore.dev/api";
interface PingParams {
  name: string;
}
interface PingResponse {
  message: string;
}
export const ping = api(
  { method: "POST" },
  async (p: PingParams): Promise<PingResponse> => {
    return { message: Hello ${p.name}! };
  }
);

Schema patterns:
- Full: api({ ... }, async (params: Params): Promise<Response> => {})
- Resp only: api({ ... }, async (): Promise<Response> => {})
- Req only: api({ ... }, async (params: Params): Promise<void> => {})
- No data: api({ ... }, async (): Promise<void> => {})

Param types:
- Header<"Header-Name">: maps field to HTTP header
- Query<type>: maps field to URL query param
- Path: maps to URL path params via :param or *wildcard
### API calls

Service-to-service calls = simple fn call syntax. Services imported from ~encore/clients. Compile-time type check + IDE autocomplete.

Example:
import { hello } from "~encore/clients";
export const myOtherAPI = api({}, async (): Promise<void> => {
  const resp = await hello.ping({ name: "World" });
  console.log(resp.message); // "Hello World!"
});
### Application structure

Core:
- Monorepo design for entire backend
- One Encore app = full app model benefits
- Supports monolith + microservices
- Services cannot nest

Service def:
import { Service } from "encore.dev/service";
export default new Service("my-service");

Patterns:

Single service (best start):
/my-app
├── package.json
├── encore.app
├── encore.service.ts    // service root
├── api.ts              // endpoints
└── db.ts               // database

Multi service:
/my-app
├── encore.app
├── hello/
│   ├── migrations/
│   ├── encore.service.ts
│   ├── hello.ts
│   └── hello_test.ts
└── world/
    ├── encore.service.ts
    └── world.ts

Large scale (systems-based):
/my-trello-clone
├── encore.app
├── trello/             // system
│   ├── board/         // service
│   └── card/          // service
├── premium/           // system
│   ├── payment/       // service
│   └── subscription/  // service
└── usr/               // system
    ├── org/           // service
    └── user/          // service
### Raw endpoints

Raw endpoints = lower-level HTTP req access via Node.js/Express.js style handling. For webhooks + custom HTTP handling.

Example:
import { api } from "encore.dev/api";
export const myRawEndpoint = api.raw(
  { expose: true, path: "/raw", method: "GET" },
  async (req, resp) => {
  resp.writeHead(200, { "Content-Type": "text/plain" });
  resp.end("Hello, raw world!");
  }
);

Usage: curl http://localhost:4000/raw → Hello, raw world!
Use: webhooks, custom HTTP resp format, direct req/resp control
### API errors

Error format:
{
    "code": "not_found",
    "message": "sprocket not found",
    "details": null
}

Impl:
import { APIError, ErrCode } from "encore.dev/api";
throw new APIError(ErrCode.NotFound, "sprocket not found");
// shorthand:
throw APIError.notFound("sprocket not found");

Error codes (name → string_value → HTTP status):
- OK → ok → 200 OK
- Canceled → canceled → 499 Client Closed Request
- Unknown → unknown → 500 Internal Server Error
- InvalidArgument → invalid_argument → 400 Bad Request
- DeadlineExceeded → deadline_exceeded → 504 Gateway Timeout
- NotFound → not_found → 404 Not Found
- AlreadyExists → already_exists → 409 Conflict
- PermissionDenied → permission_denied → 403 Forbidden
- ResourceExhausted → resource_exhausted → 429 Too Many Requests
- FailedPrecondition → failed_precondition → 400 Bad Request
- Aborted → aborted → 409 Conflict
- OutOfRange → out_of_range → 400 Bad Request
- Unimplemented → unimplemented → 501 Not Implemented
- Internal → internal → 500 Internal Server Error
- Unavailable → unavailable → 503 Unavailable
- DataLoss → data_loss → 500 Internal Server Error
- Unauthenticated → unauthenticated → 401 Unauthorized

withDetails on APIError = attach structured details returned to external clients.
### SQL databases

Encore treats SQL DBs as logical resources + natively supports PostgreSQL.

DB creation:
import { SQLDatabase } from "encore.dev/storage/sqldb";

const db = new SQLDatabase("todo", {
  migrations: "./migrations",
});

-- todo/migrations/1_create_table.up.sql --
CREATE TABLE todo_item (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  done BOOLEAN NOT NULL DEFAULT false
);

Migration naming: starts w/ number + underscore, must increase sequentially, ends w/ .up.sql (e.g., 001_first_migration.up.sql, 002_second_migration.up.sql)

DB ops (use only these methods):
- query: async iterator for multi rows
const allTodos = await db.query`SELECT * FROM todo_item`;
for await (const todo of allTodos) {
  // Process each todo
}

**Always parameterize `db.query` / `db.queryRow` w/ explicit row generic.** Without it `row.*` accesses typed `any` + silently drop into resp (e.g. `TIMESTAMPTZ` col lands as `Date` in `string`-typed resp field, no compile error). Row type fields = snake_case SQL cols, not camelCase API resp — map between yourself in loop.

const rows = await db.query<{ email: string; source_url: string; scraped_at: Date }>`
    SELECT email, source_url, created_at as scraped_at
    FROM scraped_emails
    ORDER BY created_at DESC
`;
const emails = [];
for await (const row of rows) {
    emails.push(row);
}
return { emails };

- queryRow: single row or null
async function getTodoTitle(id: number): string | undefined {
  const row = await db.queryRow`SELECT title FROM todo_item WHERE id = ${id}`;
  return row?.title;
}

- exec: inserts + queries not returning rows
await db.exec`
  INSERT INTO todo_item (title, done)
  VALUES (${title}, false)
`;

CLI: db shell (opens psql), db conn-uri (outputs conn string), db proxy (local conn proxy)

Advanced:
- Sharing DBs: export SQLDatabase from shared module or use SQLDatabase.named("name") to ref existing DB
- Extensions: pgvector, PostGIS (uses encoredotdev/postgres Docker image)
- ORM: Prisma, Drizzle (must support std SQL driver + gen std SQL files)
### Cron jobs

Encore.ts = declarative Cron Jobs for periodic + recurring tasks.

Example:
import { CronJob } from "encore.dev/cron";
import { api } from "encore.dev/api";

const _ = new CronJob("welcome-email", {
    title: "Send welcome emails",
    every: "2h",
    endpoint: sendWelcomeEmail,
})

export const sendWelcomeEmail = api({}, async () => {
    // Send welcome emails...
});

Schedule:
- every: periodic, starts midnight UTC. Interval must divide 24h evenly. Valid: 10m, 6h. Invalid: 7h
- schedule: cron exprs for complex (e.g., "0 4 15 * *" = 4am UTC on 15th)
### Pub/Sub

Async event broadcasting between services. Decouples services for reliability, improves responsiveness, cloud-agnostic.

Topics (must be package-level vars, cannot create in fns, accessible from any service):
import { Topic } from "encore.dev/pubsub"

export interface SignupEvent {
    userID: string;
}

export const signups = new Topic<SignupEvent>("signups", {
    deliveryGuarantee: "at-least-once",
});

Publish:
const messageID = await signups.publish({userID: id});

Subscriptions:
import { Subscription } from "encore.dev/pubsub";

const _ = new Subscription(signups, "send-welcome-email", {
    handler: async (event) => {
        // Send a welcome email using the event.
    },
});

Errors: failed events retried per retry policy. After max retries → dead-letter queue.

Delivery guarantees:
- at-least-once: default, possible dup, handlers must be idempotent
- exactly-once: stronger, minimized dups. Limits: AWS 300 msg/s/topic, GCP 3000+ msg/s/region. No publish-side dedup.

Message attributes (key-value for filter/order):
import { Topic, Attribute } from "encore.dev/pubsub";

export interface SignupEvent {
    userID: string;
    source: Attribute<string>;
}

Ordered delivery (delivered in order by orderingAttribute):
import { Topic, Attribute } from "encore.dev/pubsub";

export interface CartEvent {
    shoppingCartID: Attribute<number>;
    event: string;
}

export const cartEvents = new Topic<CartEvent>("cart-events", {
    deliveryGuarantee: "at-least-once",
    orderingAttribute: "shoppingCartID",
})

Limits: AWS 300 msg/s/topic, GCP 1 MBps/ordering key. No effect locally.

### Object storage

Simple + scalable file/unstructured data store.

Buckets (must be package-level vars, cannot create in fns, accessible from any service):
import { Bucket } from "encore.dev/storage/objects";

export const profilePictures = new Bucket("profile-pictures", {
  versioned: false
});

Ops:
- Upload:
const data = Buffer.from(...); // image data
const attributes = await profilePictures.upload("my-image.jpeg", data, {
  contentType: "image/jpeg",
});

- Download:
const data = await profilePictures.download("my-image.jpeg");

- List:
for await (const entry of profilePictures.list({})) {
  // Process entry
}

- Delete:
await profilePictures.remove("my-image.jpeg");

- Attrs:
const attrs = await profilePictures.attrs("my-image.jpeg");
const exists = await profilePictures.exists("my-image.jpeg");

Public access:
export const publicProfilePictures = new Bucket("public-profile-pictures", {
  public: true,
  versioned: false
});
const url = publicProfilePictures.publicUrl("my-image.jpeg");

Errors: ObjectNotFound (no such object), PreconditionFailed (upload preconditions failed), ObjectsError (base)

Bucket refs (controlled access):
Permissions: Downloader, Uploader, Lister, Attrser, Remover, ReadWriter
import { Uploader } from "encore.dev/storage/objects";
const ref = profilePictures.ref<Uploader>();
Must call from within service for proper permission tracking.
### Caching

Type-safe cache layer backed by Redis w/ auto infra provisioning.

Cluster def:
import { CacheCluster } from "encore.dev/storage/cache";

const cluster = new CacheCluster("my-cache", {
  evictionPolicy: "allkeys-lru",
});

Ref existing cluster from another service: const cluster = CacheCluster.named("my-cache");

Eviction policies: "allkeys-lru" (default), "noeviction", "allkeys-lfu", "allkeys-random", "volatile-lru", "volatile-lfu", "volatile-ttl", "volatile-random"

Keyspaces (type-safe storage, Key type + keyPattern → Redis keys):

StringKeyspace example:
import { CacheCluster, StringKeyspace, expireIn } from "encore.dev/storage/cache";

const cluster = new CacheCluster("my-cache", { evictionPolicy: "allkeys-lru" });

const tokens = new StringKeyspace<{ tokenId: string }>(cluster, {
  keyPattern: "token/:tokenId",
  defaultExpiry: expireIn(3600 * 1000),
});

await tokens.set({ tokenId: "abc123" }, "value");
const token = await tokens.get({ tokenId: "abc123" }); // undefined on miss
await tokens.delete({ tokenId: "abc123" });

IntKeyspace example:
import { IntKeyspace } from "encore.dev/storage/cache";

const counters = new IntKeyspace<{ id: string }>(cluster, {
  keyPattern: "counter/:id",
});

await counters.increment({ id: "visits" }, 1);
await counters.decrement({ id: "visits" }, 1);

StructKeyspace example:
import { StructKeyspace } from "encore.dev/storage/cache";

interface UserProfile { name: string; email: string; }

const profiles = new StructKeyspace<{ userId: string }, UserProfile>(cluster, {
  keyPattern: "profile/:userId",
  defaultExpiry: expireIn(3600 * 1000),
});

await profiles.set({ userId: "user123" }, { name: "Alice", email: "alice@example.com" });

Other keyspaces (all from "encore.dev/storage/cache"):
- FloatKeyspace<Key>: float vals. Has increment().
- StringListKeyspace<Key>: string lists. pushRight, pushLeft, popRight, popLeft, getRange, items.
- NumberListKeyspace<Key>: number lists. pushRight, pushLeft, popRight, popLeft, items.
- StringSetKeyspace<Key>: string sets. add, remove, contains, items.
- NumberSetKeyspace<Key>: number sets. add, remove, contains, items.

Expiry helpers (from "encore.dev/storage/cache"): expireIn(ms), expireInSeconds(s), expireInMinutes(m), expireInHours(h), expireDailyAt(hour, min, sec), neverExpire, keepTTL

Write opts:
await keyspace.set(key, value, { expiry: expireInMinutes(30) }); // override default
await keyspace.setIfNotExists(key, value); // only if not exists
await keyspace.replace(key, value); // only if exists, throws CacheMiss if absent

Error types: CacheMiss, CacheKeyExists (from "encore.dev/storage/cache"). get() → undefined on miss.

Local dev = in-memory Redis impl w/ ~100 key limit.
### Secrets management

Built-in secrets mgr for API keys, passwords, private keys.

Def:
import { secret } from "encore.dev/config";
const githubToken = secret("GitHubAPIToken");

Usage:
async function callGitHub() {
  const resp = await fetch("https:///api.github.com/user", {
    credentials: "include",
    headers: {
      Authorization: `token ${githubToken()}`,
    },
  });
}

Secret keys globally unique across app.

Storage methods:
- Cloud dashboard: https://app.encore.cloud → Settings → Secrets
- CLI: encore secret set --type <types> <secret-name> (types: production/prod, development/dev, preview/pr, local)
- Local override: .secrets.local.cue file (e.g., GitHubAPIToken: "my-local-override-token")

Env settings: one secret value per env type. Env-specific overrides env-type values.
### Streaming APIs

API endpoints for data streaming via WebSocket.
Stream types: StreamIn (client→server), StreamOut (server→client), StreamInOut (bidirectional)

StreamIn:
import { api } from "encore.dev/api";

interface Message {
  data: string;
  done: boolean;
}

export const uploadStream = api.streamIn<Message>(
  { path: "/upload", expose: true },
  async (stream) => {
    for await (const data of stream) {
      // Process incoming data
      if (data.done) break;
    }
  }
);

StreamOut:
export const dataStream = api.streamOut<Message>(
  { path: "/stream", expose: true },
  async (stream) => {
    // Send messages to client
    await stream.send({ data: "message" });
    await stream.close();
  }
);

StreamInOut:
export const chatStream = api.streamInOut<InMessage, OutMessage>(
  { path: "/chat", expose: true },
  async (stream) => {
    for await (const msg of stream) {
      await stream.send(/* response */);
    }
  }
);

Handshake supports: path params, query params, headers, auth data

Client usage:
const stream = client.serviceName.endpointName();
await stream.send({ /* message */ });
for await (const msg of stream) {
  // Handle incoming messages
}

Service-to-service:
import { service } from "~encore/clients";
const stream = await service.streamEndpoint();
### Validation

Built-in req validation via TS types — runtime + compile-time.

Example:
import { Header, Query, api } from "encore.dev/api";
import { Min, Max, MinLen, MaxLen, IsEmail, IsURL, StartsWith, EndsWith, MatchesRegexp } from "encore.dev/validate";

interface Request {
  limit?: Query<number>;               // Optional query parameter
  myHeader: Header<"X-My-Header">;     // Required header
  type: "sprocket" | "widget";         // Required enum in body
}

export const myEndpoint = api<Request, Response>(
  { expose: true, method: "POST", path: "/api" },
  async ({ limit, myHeader, type }) => {
    // Implementation
  }
);

Basic types: string, number, boolean, arrays (string[], number[], { name: string }[], (string | number)[]), enums ("BLOG_POST" | "COMMENT")

Modifiers:
- Optional: fieldName?: type;
- Nullable: fieldName: type | null;

Rules:
- Min/Max: count: number & (Min<3> & Max<1000>);
- MinLen/MaxLen: username: string & (MinLen<5> & MaxLen<20>);
- Format: contact: string & (IsURL | IsEmail);

Source types:
- Body: default for methods w/ req bodies, parsed from JSON body
- Query: URL query params, use Query type or default for GET/HEAD/DELETE
- Headers: HTTP headers, use Header<"Name-Of-Header"> type
- Params: URL path params (e.g., path: "/user/:id", param: { id: string })

Error resp (400 Bad Request):
{
  "code": "invalid_argument",
  "message": "unable to decode request body",
  "internal_message": "Error details"
}
### Static assets

Built-in serving for static assets (images, HTML, CSS, JS). For static sites or pre-compiled SPAs.

Basic:
import { api } from "encore.dev/api";
export const assets = api.static(
  { expose: true, path: "/frontend/*path", dir: "./assets" },
);

Serves files from ./assets under /frontend prefix. Auto-serves index.html at dir roots.

Root serving (uses !path to avoid conflicts):
export const assets = api.static(
  { expose: true, path: "/!path", dir: "./assets" },
);

Custom 404:
export const assets = api.static(
  {
    expose: true,
    path: "/!path",
    dir: "./assets",
    notFound: "./not_found.html"
  },
);
### GraphQL

Encore.ts has GraphQL via raw endpoints w/ auto tracing.

Apollo example:
import { HeaderMap } from "@apollo/server";
import { api } from "encore.dev/api";
const { ApolloServer, gql } = require("apollo-server");
import { json } from "node:stream/consumers";

const server = new ApolloServer({ typeDefs, resolvers });
await server.start();

export const graphqlAPI = api.raw(
  { expose: true, path: "/graphql", method: "*" },
  async (req, res) => {
    server.assertStarted("/graphql");

    const headers = new HeaderMap();
    for (const [key, value] of Object.entries(req.headers)) {
      if (value !== undefined) {
        headers.set(key, Array.isArray(value) ? value.join(", ") : value);
      }
    }

    const httpGraphQLResponse = await server.executeHTTPGraphQLRequest({
      httpGraphQLRequest: {
        headers,
        method: req.method!.toUpperCase(),
        body: await json(req),
        search: new URLSearchParams(req.url ?? "").toString(),
      },
      context: async () => ({ req, res }),
    });

    // Set response headers and status
    for (const [key, value] of httpGraphQLResponse.headers) {
      res.setHeader(key, value);
    }
    res.statusCode = httpGraphQLResponse.status || 200;

    // Write response
    if (httpGraphQLResponse.body.kind === "complete") {
      res.end(httpGraphQLResponse.body.string);
      return;
    }

    for await (const chunk of httpGraphQLResponse.body.asyncIterator) {
      res.write(chunk);
    }
    res.end();
  }
);

REST integration example:
Schema:
type Query {
  books: [Book]
}
type Book {
  title: String!
  author: String!
}

Resolver:
import { book } from "~encore/clients";
import { QueryResolvers } from "../__generated__/resolvers-types";

const queries: QueryResolvers = {
  books: async () => {
    const { books } = await book.list();
    return books;
  },
};

REST endpoint:
import { api } from "encore.dev/api";
import { Book } from "../__generated__/resolvers-types";

export const list = api(
  { expose: true, method: "GET", path: "/books" },
  async (): Promise<{ books: Book[] }> => {
    return { books: db };
  }
);
### Authentication

Auth system for identifying API callers. Enable: set auth: true in endpoint options.

Auth handler:
import { Header, Gateway } from "encore.dev/api";
import { authHandler } from "encore.dev/auth";

interface AuthParams {
    authorization: Header<"Authorization">;
}

interface AuthData {
    userID: string;
}

export const auth = authHandler<AuthParams, AuthData>(
    async (params) => {
        // Authenticate user based on params
        return {userID: "my-user-id"};
    }
)

export const gateway = new Gateway({
    authHandler: auth,
})

Reject: throw APIError.unauthenticated("bad credentials");

Auth process:
1. Determine auth: triggers on any req w/ auth params. Returns AuthData (success), throws Unauthenticated (treated as no auth), or throws other error (req aborted).
2. Endpoint call: if endpoint needs auth + req unauth → reject. If authed, auth data passed regardless of requirements.

Auth data access: import getAuthData from ~encore/auth for type-safe resolution. Auto propagates in internal API calls. Calls to auth-required endpoints fail if orig req lacks auth.
### Metadata

Access env + app info via metadata API from encore.dev.

appMeta() returns: appId (app name), apiBaseUrl (public API URL), environment (current env), build (VCS revision), deploy (deploy ID + timestamp)

currentRequest() returns:
- API call:
interface APICallMeta {
  type: "api-call";
  api: APIDesc;
  method: Method;
  path: string;
  pathAndQuery: string;
  pathParams: Record<string, any>;
  headers: Record<string, string | string[]>;
  parsedPayload?: Record<string, any>;
}

- PubSub:
interface PubSubMessageMeta {
  type: "pubsub-message";
  service: string;
  topic: string;
  subscription: string;
  messageId: string;
  deliveryAttempt: number;
  parsedPayload?: Record<string, any>;
}

Returns undefined if called during service init.

Use cases:
import { appMeta } from "encore.dev";

// Cloud-specific behavior
async function audit(userID: string, event: Record<string, any>) {
  const cloud = appMeta().environment.cloud;
  switch (cloud) {
    case "aws": return writeIntoRedshift(userID, event);
    case "gcp": return writeIntoBigQuery(userID, event);
    case "local": return writeIntoFile(userID, event);
    default: throw new Error(`unknown cloud: ${cloud}`);
  }
}

// Environment-specific behavior
switch (appMeta().environment.type) {
  case "test":
  case "development":
    await markEmailVerified(userID);
    break;
  default:
    await sendVerificationEmail(userID);
    break;
}
### Middleware

Reusable code running before/after API reqs across endpoints.

Basic:
import { middleware } from "encore.dev/api";

export default new Service("myService", {
    middlewares: [
        middleware({ target: { auth: true } }, async (req, next) => {
            // Pre-handler logic
            const resp = await next(req);
            // Post-handler logic
            return resp
        })
    ]
});

Req access types:
- Typed API: req.requestMeta
- Streaming: req.requestMeta, req.stream
- Raw: req.rawRequest, req.rawResponse

Resp handling (HandlerResponse):
resp.header.set(key, value)
resp.header.add(key, value)

Ordering (executes in definition order):
export default new Service("myService", {
    middlewares: [
        first,
        second,
        third
    ],
});

Targeting: use target option instead of runtime filter for perf. Defaults to all endpoints if target unset.
### ORM integration

Built-in ORM + migration framework support via named DBs + SQL migration files.
Requires: ORM must support std SQL driver. Migration framework must gen std SQL files.

DB conn:
import { SQLDatabase } from "encore.dev/storage/sqldb";

const SiteDB = new SQLDatabase("siteDB", {
  migrations: "./migrations",
});

const connStr = SiteDB.connectionString;
### Drizzle integration

DB setup (database.ts):
import { api } from "encore.dev/api";
import { SQLDatabase } from "encore.dev/storage/sqldb";
import { drizzle } from "drizzle-orm/node-postgres";
import { users } from "./schema";

const db = new SQLDatabase("test", {
  migrations: {
    path: "migrations",
    source: "drizzle",
  },
});

const orm = drizzle(db.connectionString);
await orm.select().from(users);

Drizzle config (drizzle.config.ts):
import 'dotenv/config';
import { defineConfig } from 'drizzle-kit';

export default defineConfig({
  out: 'migrations',
  schema: 'schema.ts',
  dialect: 'postgresql',
});

Schema (schema.ts):
import * as p from "drizzle-orm/pg-core";

export const users = p.pgTable("users", {
  id: p.serial().primaryKey(),
  name: p.text(),
  email: p.text().unique(),
});

Gen migrations: drizzle-kit generate (run in dir w/ drizzle.config.ts)
Migrations auto-applied at Encore runtime.
### CORS

CORS controls which origins can access API. Scope: browser reqs to resources on different origins (scheme, domain, port).

Config in encore.app under global_cors:
- debug: bool, CORS debug logging
- allow_headers: string[], additional accepted headers ("*" = all)
- expose_headers: string[], additional exposed headers ("*" = all)
- allow_origins_without_credentials: string[], allowed origins for non-cred reqs (default: ["*"])
- allow_origins_with_credentials: string[], allowed origins for credentialed (wildcards: https://*.example.com, https://*-myapp.example.com)

Defaults: allow unauth reqs from all origins, disallow auth reqs from other origins, all origins allowed in local dev.

Header handling: Encore auto-configures headers via static analysis. Additional headers configurable via allow_headers + expose_headers for custom headers in raw endpoints.
### Logging

Built-in structured logging — free-form msgs + type-safe key-value pairs.

import log from "encore.dev/log";

Levels: error, warn, info, debug, trace

Basic:
log.info("log message", {is_subscriber: true})
log.error(err, "something went terribly wrong!")

With context (group logs w/ shared key-value pairs):
const logger = log.with({is_subscriber: true})
logger.info("user logged in", {login_method: "oauth"}) // includes is_subscriber=true
### Testing

Encore.ts uses std TS testing tools. Recommended: Vitest.

Setup:
npm install -D vitest

Add to package.json:
{
  "scripts": {
    "test": "vitest"
  }
}

Run tests:
- encore test: recommended — auto sets up test DBs, isolated infra per test, handles service deps
- npm test: direct, no infra setup

Test API endpoint:
import { describe, it, expect } from "vitest";
import { hello } from "./api";

describe("hello endpoint", () => {
  it("returns a greeting", async () => {
    const response = await hello();
    expect(response.message).toBe("Hello, World!");
  });
});

Test w/ req params:
import { describe, it, expect } from "vitest";
import { getUser } from "./api";

describe("getUser endpoint", () => {
  it("returns the user by ID", async () => {
    const user = await getUser({ id: "123" });
    expect(user.id).toBe("123");
    expect(user.name).toBeDefined();
  });
});

Test DB ops (Encore gives isolated test DBs):
import { describe, it, expect, beforeEach } from "vitest";
import { createUser, getUser, db } from "./user";

describe("user operations", () => {
  beforeEach(async () => {
    await db.exec`DELETE FROM users`;
  });

  it("creates and retrieves a user", async () => {
    const created = await createUser({ email: "test@example.com", name: "Test" });
    const retrieved = await getUser({ id: created.id });
    expect(retrieved.email).toBe("test@example.com");
  });
});

Test error cases:
import { describe, it, expect } from "vitest";
import { getUser } from "./api";
import { APIError } from "encore.dev/api";

describe("error handling", () => {
  it("throws NotFound for missing user", async () => {
    await expect(getUser({ id: "nonexistent" }))
      .rejects
      .toThrow("user not found");
  });

  it("throws with correct error code", async () => {
    try {
      await getUser({ id: "nonexistent" });
    } catch (error) {
      expect(error).toBeInstanceOf(APIError);
      expect((error as APIError).code).toBe("not_found");
    }
  });
});

Test Pub/Sub:
import { describe, it, expect } from "vitest";
import { orderCreated } from "./events";

describe("pub/sub", () => {
  it("publishes order created event", async () => {
    const messageId = await orderCreated.publish({
      orderId: "order-123",
      userId: "user-456",
      total: 9999,
    });
    expect(messageId).toBeDefined();
  });
});

Test Cron Jobs (test underlying fn, not cron schedule):
import { describe, it, expect } from "vitest";
import { cleanupExpiredSessions } from "./cleanup";

describe("cleanup job", () => {
  it("removes expired sessions", async () => {
    await createExpiredSession();
    await cleanupExpiredSessions();
    const remaining = await countSessions();
    expect(remaining).toBe(0);
  });
});

Vitest config (vitest.config.ts):
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
    include: ["**/*.test.ts"],
    coverage: {
      reporter: ["text", "json", "html"],
    },
  },
});

Guidelines:
- Use `encore test` to run w/ infra setup
- Each test file gets isolated DB transaction (rolled back after)
- Test API endpoints by calling directly as fns
- Service-to-service calls work normally in tests
- Mock external deps (third-party APIs, email services)
- Don't mock Encore infra (DBs, Pub/Sub) — use real thing
### Encore Toolbar (frontend dev panel)

Dev-only browser panel: intercepts frontend HTTP calls, captures Encore trace IDs, jumps to traces in the dashboard. Install: add `<script src="https://encore.dev/encore-toolbar.js"></script>` early in `<head>` (no `async`/`defer`). Reach for it when a frontend talks to an Encore backend.
### Example apps

- Hello World: https://github.com/encoredev/examples/tree/main/ts/hello-world
- URL Shortener: https://github.com/encoredev/examples/tree/main/ts/url-shortener
- Uptime Monitor: https://github.com/encoredev/examples/tree/main/ts/uptime
### Package management

Default: single root-level package.json (monorepo) for Encore.ts projects including frontend deps.
Alt: separate package.json in sub-packages, but Encore.ts app must use one package w/ single package.json, other separate packages must be pre-transpiled to JS.
## Encore CLI reference

Execution:
- encore help [command]: lists cmds (or shows usage for specific cmd). Use when unsure of subcommand/flag.
- encore run [--debug] [--watch=true] [-p port] [flags]: runs app. Before starting yourself, check if already running by polling `http://localhost:<port>/__encore/healthz` (default `4000`) — `200` = healthy, hit endpoints directly w/o re-running.
- encore check ['curl ...'; 'curl ...'] | encore check < commands.txt: compile + boot app, verify services healthy, optionally run curls. See section above.
- encore test [flags]: provisions isolated test infra (DBs, Pub/Sub, etc.) then runs `test` script from `package.json` (Vitest recommended). Test DBs skip fsync + use in-memory FS for speed. Use instead of `npm test` / `vitest` direct when code touches Encore infra.
- encore exec path/to/script [args...]: run executable script against local app.

App mgmt:
- encore app clone [app-id] [directory]: clone Encore app
- encore app create [name] [--example=name] [-l ts]: create new Encore app
- encore app init [name]: create new app from existing repo
- encore app link [app-id] [-f]: link app w/ server

Auth:
- encore auth login [-k auth-key]: log in
- encore auth logout: log out
- encore auth signup: create account
- encore auth whoami: show current user

Daemon:
- encore daemon: restart daemon for unexpected behavior
- encore daemon env: output env info

DB:
- encore db shell database-name [--env=name]: connect via psql (--write, --admin, --superuser flags)
- encore db conn-uri database-name [--env=name]: output conn string
- encore db proxy [--env=name]: local DB conn proxy
- encore db reset <db-names...|--all>: reset specified DBs

Code gen:
- encore gen client [app-id] [--env=name] [--lang=lang]: gen API client (langs: go, typescript, javascript, openapi)

Logs:
- encore logs [--env=prod] [--json] [-q]: stream app logs

K8s:
- encore k8s configure --env=ENV_NAME: update kubectl config for env

Secrets:
- encore secret set --type types <secret-name>: set secret (types: production/prod, development/dev, preview/pr, local)
- encore secret set --env <env-name> <secret-name>: set env-specific secret
- encore secret list [keys...]: list secrets
- encore secret delete <id>: delete secret value

Namespaces (infra envs, alias `encore ns`):
- encore namespace list [--output=columns|json]: list infra namespaces
- encore namespace create NAME: create new namespace
- encore namespace switch [--create] NAME: switch active namespace
- encore namespace delete NAME: delete namespace

MCP (programmatic access to the local app — same surface as `.mcp.json`):
- encore mcp run: stdio-based MCP session
- encore mcp start: SSE-based MCP session; prints the SSE URL

Config:
- encore config <key> [<value>] [--app|--global|--all]: get/set CLI config

Telemetry:
- encore telemetry: status
- encore telemetry enable / disable: toggle telemetry reporting

LLM rules:
- encore llm-rules init: generate LLM rule files for this project

Random data (utility):
- encore rand uuid [-1|-4|-6|-7]: gen UUID (default v4)
- encore rand bytes N [-f format]: gen N random bytes
- encore rand words [--sep=SEP] NUM: gen memorable passphrase

Deploy (alpha):
- encore alpha deploy --env=<name> (--commit=<sha> | --branch=<name>): deploy app to a cloud env

Version:
- encore version: report current
- encore version update: check + apply updates

Build:
- encore build docker IMAGE_TAG [--base string] [--push] [--cgo] [--os] [--arch]: build portable Docker image