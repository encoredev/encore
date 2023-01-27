import Code from "~c/snippets/Code";
import React from "react";

export interface SnippetSection {
  slug: string;
  heading: string;
  subSections: {
    heading: string;
    content: JSX.Element;
  }[];
}

const apiSection: SnippetSection = {
  slug: "api",
  heading: "APIs",
  subSections: [
    {
      heading: "Defining APIs",
      content: (
        <Code
          lang="go"
          rawContents={`
package hello // service name

//encore:api public
func Ping(ctx context.Context, params *PingParams) (*PingResponse, error) {
    msg := fmt.Sprintf("Hello, %s!", params.Name)
    return &PingResponse{Message: msg}, nil
}
`.trim()}
        />
      ),
    },
    {
      heading: "Defining Request and Response schemas",
      content: (
        <Code
          lang="go"
          rawContents={`
// PingParams is the request data for the Ping endpoint.
type PingParams struct {
    Name string
}

// PingResponse is the response data for the Ping endpoint.
type PingResponse struct {
    Message string
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
      heading: "Receive Webhooks",
      content: (
        <>
          <Code
            lang="go"
            rawContents={`
import "net/http"

// Webhook receives incoming webhooks from Some Service That Sends Webhooks.
//encore:api public raw
func Webhook(w http.ResponseWriter, req *http.Request) {
    // ... operate on the raw HTTP request ...
}
`.trim()}
          />
          <p className="mt-5">
            <b>Hint:</b> Like any other API endpoint, this will be exposed at:{" "}
            <code>{"https://<env>-<app-id>.encr.app/service.Webhook"}</code>
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
      heading: "Querying a database",
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
    ID int64
    Title string
    Done bool
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
  ],
};

export const snippetData: SnippetSection[] = [apiSection, databaseSection];
