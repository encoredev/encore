import {
  ChatBubbleLeftRightIcon,
  CircleStackIcon,
  ClockIcon,
  CodeBracketIcon,
  EnvelopeIcon,
  KeyIcon,
  Square3Stack3DIcon,
} from "@heroicons/react/24/solid";
import React from "react";
import Code from "~c/snippets/Code";

export interface SnippetSection {
  slug: string;
  heading: string;
  description?: JSX.Element | string;
  icon: typeof CircleStackIcon;
  subSections: {
    heading: string;
    content: JSX.Element;
  }[];
}

const extLink = (href: string, content: string | JSX.Element) => {
  return (
    <a href={href} target="_blank" className="link-brandient">
      {content}
    </a>
  );
};

const docLink = (path: string) => {
  return <span>{extLink(`https://encore.dev/docs${path}`, "Read the docs")} to learn more.</span>;
};

const apiSection: SnippetSection = {
  slug: "api",
  heading: "APIs",
  description: (
    <>
      Encore lets you define APIs as regular Go functions with an annotation. API calls are made as
      regular function calls. {docLink("/primitives/services-and-apis")}
    </>
  ),
  icon: ChatBubbleLeftRightIcon,
  subSections: [
    {
      heading: "Defining APIs",
      content: (
        <Code
          lang="go"
          rawContents={`
//encore:api public method=POST path=/ping
func Ping(ctx context.Context, params *PingParams) (*PingResponse, error) {
    msg := fmt.Sprintf("Hello, %s!", params.Name)
    return &PingResponse{Message: msg}, nil
}
`.trim()}
        />
      ),
    },
    {
      heading: "Calling APIs",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "encore.app/hello" // import service

//encore:api public
func MyOtherAPI(ctx context.Context) error {
    resp, err := hello.Ping(ctx, &hello.PingParams{Name: "World"})
    if err == nil {
        log.Println(resp.Message) // "Hello, World!"
    }
    return err
}
`.trim()}
          />
          <p className="mt-5">
            <b>Hint:</b> Import the service package and call the API endpoint using a regular
            function call.
          </p>
        </>
      ),
    },
    {
      heading: "Raw endpoints",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "net/http"

// A raw endpoint operates on standard HTTP requests.
// It's great for things like Webhooks, WebSockets, and GraphQL.
//encore:api public raw
func Webhook(w http.ResponseWriter, req *http.Request) {
    // ... operate on the raw HTTP request ...
}
`.trim()}
          />
        </>
      ),
    },
    {
      heading: "GraphQL",
      content: (
        <>
          <p>
            Encore supports GraphQL servers through raw endpoints. We recommend using{" "}
            {extLink("https://gqlgen.com", "gqlgen")}.
          </p>
          <p className="mt-5">
            An example of using GraphQL with Encore{" "}
            {extLink(
              "https://github.com/encoredev/examples/tree/main/graphql",
              "can be found here"
            )}
            .
          </p>
        </>
      ),
    },
  ],
};

/** ----- Databases -----  **/

const databaseSection: SnippetSection = {
  slug: "database",
  heading: "Databases",
  description: (
    <>
      Here are some code snippets for using SQL databases with Encore.{" "}
      {docLink("/primitives/databases")}
    </>
  ),
  icon: CircleStackIcon,
  subSections: [
    {
      heading: "Defining a SQL database",
      content: (
        <div className="space-y-5">
          <p>
            Database schemas are defined by creating migration files, Encore automatically runs
            migrations and provisions the necessary infrastructure.
          </p>
          <Code
            lang="go"
            rawContents={`
/my-app
└── todo                             // todo service (a Go package)
    ├── migrations                   // todo service db migrations (directory)
    │   ├── 1_create_table.up.sql    // todo service db migration
    │   └── 2_add_field.up.sql       // todo service db migration
    └── todo.go                      // todo service code
`.trim()}
          />
          <p>The first migration usually defines the initial table structure, for example:</p>
          <Code
            lang="sql"
            rawContents={`
CREATE TABLE todo_item (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    done BOOLEAN NOT NULL DEFAULT false
);
`.trim()}
          />
        </div>
      ),
    },
    {
      heading: "Inserting data into a database",
      content: (
        <div className="space-y-5">
          <p>
            One way of inserting data is with a helper function that uses the package function
            <code>sqldb.Exec</code>:
          </p>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/storage/sqldb"

// insert inserts a todo item into the database.
func insert(ctx context.Context, id, title string, done bool) error {
\t_, err := sqldb.Exec(ctx, \`
\t\tINSERT INTO todo_item (id, title, done)
\t\tVALUES ($1, $2, $3)
\t\`, id, title, done)
\treturn err
}
`.trim()}
          />
        </div>
      ),
    },
    {
      heading: "Reading a single row",
      content: (
        <div className="space-y-5">
          <p>
            To read a single todo item in the example schema above, we can use{" "}
            <code>sqldb.QueryRow</code>:
          </p>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/storage/sqldb"

var item struct {
    ID    int64
    Title string
    Done  bool
}
err := sqldb.QueryRow(ctx, \`
    SELECT id, title, done
    FROM todo_item
    LIMIT 1
\`).Scan(&item.ID, &item.Title, &item.Done)
`.trim()}
          />
          <p>
            <b>Hint:</b> If <code>sqldb.QueryRow</code> does not find a matching row, it reports an
            error that can be checked against by importing the standard library <code>errors</code>{" "}
            package and calling
            <code>errors.Is(err, sqldb.ErrNoRows)</code>.
          </p>
        </div>
      ),
    },
    {
      heading: "Reading multiple rows",
      content: (
        <div className="space-y-5">
          <p>
            To read multiple todo items in the example schema above, we can use{" "}
            <code>sqldb.Query</code>:
          </p>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/storage/sqldb"

type item struct {
    ID    int64
    Title string
    Done  bool
}

rows, err := sqldb.Query(ctx, "SELECT id, title, done FROM todo_item")
if err != nil {
    return nil, err
}
defer rows.Close() // IMPORTANT: close the database cursor when we're done

var items []item
for rows.Next() {
    var it item
    if err := rows.Scan(&it.ID, &it.Title, &it.Done); err != nil {
      return nil, err
    }
    items = append(items, it)
}
if err := rows.Err(); err != nil {
    return nil, err
}
return items, nil
`.trim()}
          />
        </div>
      ),
    },
  ],
};

/** ----- Cron Jobs -----  **/

const cronJobsSection: SnippetSection = {
  slug: "cron",
  heading: "Cron Jobs",
  icon: ClockIcon,
  description: (
    <>
      Cron Jobs are periodic tasks that automatically calls a predefined API endpoint on a schedule.{" "}
      {docLink("/primitives/cron-jobs")}
    </>
  ),
  subSections: [
    {
      heading: "With basic schedule",
      content: (
        <Code
          lang="go"
          rawContents={`
import "encore.dev/cron"

// Define a cron job to send welcome emails every 2 hours.
var _ = cron.NewJob("welcome-email", cron.JobConfig{
	Title:    "Send welcome emails",
	Every:    2 * cron.Hour,
	Endpoint: SendWelcomeEmail,
})
`.trim()}
        />
      ),
    },
    {
      heading: "With cron expression",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/cron"

// Define a cron job to sync accounting data every night at 4:05am UTC.
var _ = cron.NewJob("accounting-sync", cron.JobConfig{
	Title:    "Sync accounting data",
	Schedule: "5 4 * * *"
	Endpoint: SyncAccountingData,
})
`.trim()}
          />
          <p className="mt-5">
            <b>Note:</b> Cron schedule syntax is complex. See{" "}
            {extLink("https://crontab.guru", "crontab.guru")} for an explanation of cron schedule
            expressions.
          </p>
        </>
      ),
    },
  ],
};

/** ----- PubSub -----  **/

const pubSubSection: SnippetSection = {
  slug: "pubsub",
  heading: "Pub/Sub",
  description: (
    <>
      PubSub lets you build systems that communicate by broadcasting events asynchronously.
      {docLink("/primitives/pubsub")}
    </>
  ),
  icon: EnvelopeIcon,
  subSections: [
    {
      heading: "Define a topic",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/pubsub"

type Event struct {
    // ... fields ...
}

var TopicName = pubsub.NewTopic[*Event]("topic-name", pubsub.TopicConfig{
    DeliveryGuarantee: pubsub.AtLeastOnce,
})
`.trim()}
          />
          <div className="mt-5">
            <b>Hint:</b> Publish messages to the topic with{" "}
            <code>{`TopicName.Publish(ctx, &Event{})`}</code>
          </div>
        </>
      ),
    },
    {
      heading: "Define a subscriber",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/pubsub"

var _ = pubsub.NewSubscription(TopicName, "subscription-name",
    pubsub.SubscriptionConfig[*Event]{
        Handler: func(ctx context.Context, event *Event) error {
          // ...handle message...
        },
    },
)
`.trim()}
          />
          <p className="mt-5">
            <b>Note:</b> The <code>Handler</code> function can be called concurrently by multiple
            goroutines if there are several messages to process.
          </p>
          <p className="mt-5">
            <b>Note:</b> If the <code>Handler</code> returns an error the message will be retried,
            and eventually moved to a Dead Letter Queue (DLQ).
          </p>
        </>
      ),
    },
    {
      heading: "Customize retry behavior",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "time"
import "encore.dev/pubsub"

var _ = pubsub.NewSubscription(TopicName, "subscription-name",
    pubsub.SubscriptionConfig[*Event]{
        AckDeadline:      60 * time.Second,
        MessageRetention: 14 * 24 * time.Hour, // ~14 days
        RetryPolicy:      &pubsub.RetryPolicy{
            MinBackoff: 5 * time.Second,
            MaxBackoff: 30 * time.Minute,
            MaxRetries: 10,
        },

        Handler: func(ctx context.Context, event *Event) error {
          // ...handle message...
        },
    },
)
`.trim()}
          />
        </>
      ),
    },
  ],
};

/** ----- Cache -----  **/

const cacheSection: SnippetSection = {
  slug: "cache",
  heading: "Cache",
  description: (
    <>
      Here are some snippets for using Encore's cache functionality.
      {docLink("/primitives/caching")}
    </>
  ),
  icon: Square3Stack3DIcon,
  subSections: [
    {
      heading: "Define a cache cluster",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/storage/cache"

var MyCacheCluster = cache.NewCluster("my-cache-cluster", cache.ClusterConfig{
    // EvictionPolicy tells Redis how to evict keys when the cache reaches
    // its memory limit. For typical cache use cases, cache.AllKeysLRU is a good default.
    EvictionPolicy: cache.AllKeysLRU,
})

`.trim()}
          />
          <div className="mt-5">
            <b>Note:</b> When starting out it's recommended to use a single cache cluster that's
            shared between different your services.
          </div>
        </>
      ),
    },
    {
      heading: "Use basic keyspaces",
      content: (
        <>
          <div className="mb-5 flex flex-col gap-5">
            <p>
              Encore comes with a full suite of keyspace types, each with a wide variety of cache
              operations. Basic keyspace types include{" "}
              {extLink("https://pkg.go.dev/encore.dev/storage/cache#NewStringKeyspace", "strings")},{" "}
              {extLink("https://pkg.go.dev/encore.dev/storage/cache#NewIntKeyspace", "integers")},{" "}
              {extLink("https://pkg.go.dev/encore.dev/storage/cache#NewFloatKeyspace", "floats")},{" "}
              and{" "}
              {extLink(
                "https://pkg.go.dev/encore.dev/storage/cache#NewStructKeyspace",
                "struct types"
              )}
              .
            </p>
            <p>
              There are also more advanced keyspaces for storing sets of basic types and ordered
              lists of basic types. These keyspaces offer a different, specialized set of methods
              specific to set and list operations.
            </p>
            <p>
              For a list of the supported operations, see the{" "}
              {extLink("https://pkg.go.dev/encore.dev/storage/cache", "package documentation")}.
            </p>
          </div>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/cache"
import "encore.dev/beta/auth"

type MyKey struct {
  UserID       auth.UID
  ResourcePath string // the resource being accessed
}

// RequestsPerUser caches the number of requests to a given resource by user
// within a 10 second window.
var RequestsPerUser = cache.NewIntKeyspace[MyKey](cluster, cache.KeyspaceConfig{
    KeyPattern:    "requests/:UserID/:ResourcePath",
    DefaultExpiry: cache.ExpireIn(10 * time.Second),
})
`.trim()}
          />
        </>
      ),
    },
    {
      heading: "Use list keyspaces",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/cache"

type MyKey struct {
  UserID       auth.UID
  ResourcePath string // the resource being accessed
}

// MyKeyspace caches lists of strings, keyed by integers.
var MyKeyspace = cache.NewListKeyspace[int, string](cluster, cache.KeyspaceConfig{
    KeyPattern:    "path/:key",
})
`.trim()}
          />
        </>
      ),
    },
  ],
};

/** ----- Secrets -----  **/

const secretsSection: SnippetSection = {
  slug: "secrets",
  heading: "Secrets",
  description: (
    <>
      Here are some snippets for using Encore's secrets management functionality.
      {docLink("/primitives/secrets")}
    </>
  ),
  icon: KeyIcon,
  subSections: [
    {
      heading: "Setting a secret",
      content: (
        <>
          <Code
            lang="bash"
            rawContents={`
      
# Set a secret for all environments at once
encore secret set --type prod,dev,local,pr MySecretKey

# Set a secret for only local development
encore secret set --type local MySecretKey

# Set a production secret to the contents of a JSON file
encore secret set --type prod MySecretKey < path/to/file.json
`.trim()}
          />
        </>
      ),
    },
    {
      heading: "Using a secret",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
var secrets struct {
    MySecretKey string
}
`.trim()}
          />
          <p className="mt-5">
            <b>Note:</b> Encore specially recognizes this pattern and ensures all the secrets
            defined are set. Each field inside the <code>secrets</code> struct is a separate secret,
            and must be of type <code>string</code>.
          </p>
        </>
      ),
    },
  ],
};

/** ----- Config -----  **/

const configSection: SnippetSection = {
  slug: "config",
  heading: "Configuration",
  description: (
    <>
      Here are some snippets for using Encore's config functionality.
      {docLink("/primitives/config")}
    </>
  ),
  icon: CodeBracketIcon,
  subSections: [
    {
      heading: "Defining config schema",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "encore.dev/config"

// Define config schema
type Config struct {
  ReadOnly config.Bool // true if the system is in read-only mode
}

// Load the config
var cfg = config.Load[*Config]()
`.trim()}
          />
          <p className="mt-5">
            <b>Note:</b> With this code, Encore automatically generates a CUE schema file named{" "}
            <code>encore.gen.cue</code>. This file is auto-generated and should not be modified. Add
            your configuration values to a separate CUE file of your choice.
          </p>
        </>
      ),
    },
    {
      heading: "Writing config values",
      content: (
        <>
          <Code
            lang="cue"
            rawContents={`
// Default ReadOnly to false
ReadOnly: bool | *false

// Set ReadOnly to true for ephemeral environments (aka Preview Environments).
if #Meta.Environment.Type == "ephemeral" {
  ReadOnly: true
}
`.trim()}
          />
          <p className="mt-5">
            <b>Note:</b> Add this code to a separate <code>.cue</code> file after having defined the
            config schema (see above).
          </p>
        </>
      ),
    },
  ],
};

export const snippetData: SnippetSection[] = [
  apiSection,
  cacheSection,
  configSection,
  cronJobsSection,
  databaseSection,
  pubSubSection,
  secretsSection,
];
