---
seotitle: Migrate from Express to Encore.ts
seodesc: Learn how migrate your Express.js app over to use Encore.ts for better performance and improved development tools.
title: Migrating from Express.js
lang: ts
---

If you have an existing app using [Express.js](https://expressjs.com/) and want to migrate it to Encore.ts, this guide is
for you. This guide can also serve as a comparison between the two frameworks.

<iframe width="560" height="315" class="aspect-video" src="https://www.youtube.com/embed/hA9syK_FtZw?si=EScQ-x3qOLdImrMb" title="Express.js vs Encore.ts" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe>

## Why migrate to Encore.ts?

Express.js is a great choice for building simple APIs, but as your application grows you will likely run into limitations. There is a large community around Express.js, providing many plugins and middleware to work around these limitations. However relying heavily on plugins can make it hard to find the right tools for your use case. It also means that you will need to maintain a lot of dependencies.

Encore.ts is a framework that aims to make it easier to build robust and type-safe backends with
TypeScript. Encore.ts has 0 npm dependencies, is built with performance in mind, and has a lot of built-in features for building production ready backends. You can deploy an Encore.ts app to any hosting service that accepts Docker containers, or use [Encore Cloud](/use-cases/devops-automation) to fully automate your DevOps and infrastructure.

### Performance

Unlike a lot of other Node.js frameworks, Encore.ts is not built on top of Express.js. Instead, Encore.ts has its own
high-performance runtime, with a multi-threaded, asynchronous event loop written in Rust. The Encore Runtime handles all I/O like accepting and processing incoming HTTP requests. This runs as a completely independent event loop that utilizes as many threads as the underlying hardware supports. The result of this is that Encore.ts performs
**9x faster** than Express.js. Learn more about the [Encore.ts Runtime](/blog/event-loops).

### Built-in benefits

When using Encore.ts you get a lot of built-in features without having to install any additional dependencies:

| Built-in benefits                                      |                       <!-- -->                       |                                                       <!-- --> |
| :----------------------------------------------------- | :--------------------------------------------------: | -------------------------------------------------------------: |
| [Pub/Sub integrations](/docs/ts/primitives/pubsub)     |  [Type-safe API schemas](/docs/ts/primitives/apis)   |        [API Client generation](/docs/ts/cli/client-generation) |
| [Secrets management](/docs/ts/primitives/secrets)      |        [CORS handling](/docs/ts/develop/cors)        | [Local Development Dashboard](/docs/ts/observability/dev-dash) |
| [Database integrations](/docs/ts/primitives/databases) | [Architecture Diagrams](/docs/ts/observability/flow) |      [Service Catalog](/docs/ts/observability/service-catalog) |
| [Request validation](/blog/event-loops)                |      [Cron Jobs](/docs/ts/primitives/cron-jobs)      |                [Local tracing](/docs/ts/observability/tracing) |

## Migration guide

Below we've outlined two main strategies you can use to migrate your existing Express.js application to Encore.ts. Pick the strategy that best suits your situation and application.

<GitHubLink
href="https://github.com/encoredev/examples/tree/main/ts/expressjs-migration"
desc="Code examples for migrating an Express.js app to Encore.ts"
/>

<Accordion>

### Forklift migration (quick start)

When you quickly want to migrate to Encore.ts and don't need all the functionality to begin with, you can use a forklift migration strategy. This approach moves the entire application over to Encore.ts in one shot, by wrapping your existing HTTP router in a catch-all handler.

**Approach benefits**

- You can get your application up and running with Encore.ts quickly and start moving features over to Encore.ts while the rest of the application is still untouched.
- You will see a partial performance boost right away because the HTTP layer is now running on the Encore Rust runtime. But to get the full performance benefits, you will need to start using Encore's [API declarations](/docs/ts/primitives/defining-apis) and [infrastructure declarations](/docs/ts#explore-how-to-use-each-backend-primitive).

**Approach drawbacks**

- Because all requests will be proxied through the catch-all handler, you will not be able to get all the benefits from the [distributed tracing](/docs/ts/observability/tracing), which rely on the [Encore application model](/docs/ts/concepts/application-model).
- [Encore Flow](/docs/ts/observability/flow) and the [Service Catalog](/docs/ts/observability/service-catalog) will not be able to show you the full picture of your application until you start moving services and APIs over to Encore.ts.
- You will not be able to use the [API Client generation](/docs/ts/cli/client-generation) feature until you start defining APIs in Encore.ts.

#### 1. Install Encore

If this is the first time you're using Encore, you first need to install the CLI that runs the local development
environment. Use the appropriate command for your system:

- **macOS:** `brew install encoredev/tap/encore`
- **Linux:** `curl -L https://encore.dev/install.sh | bash`
- **Windows:** `iwr https://encore.dev/install.ps1 | iex`

[Installation docs](https://encore.dev/docs/install)

#### 2. Add Encore.ts to your project

```bash
npm i encore.dev
```

#### 3. Initialize an Encore app

Inside your project directory, run the following command to create an Encore app:

```bash
encore app init
```

This will create an `encore.app` file in the root of your project.

#### 4. Configure your tsconfig.json

To the `tsconfig.json` file in the root of your project, add the following:

```json
-- tsconfig.json --
{
  "compilerOptions": {
    "paths": {
      "~encore/*": [
        "./encore.gen/*"
      ]
    }
  }
}
```

When Encore.ts is parsing your code it will specifically look for `~encore/*` imports.

#### 5. Define an Encore.ts service

When running an app using Encore.ts you need at least one [Encore service](/docs/ts/primitives/services). Apart from that, Encore.ts in not opinionated in how you structure your code, you are free to go with a monolith or microservice approach. Learn more in our [App Structure docs](/docs/ts/primitives/app-structure).

In the root of your App, add a file named `encore.service.ts`. The file must export a service instance, by calling
`new Service`, imported from `encore.dev/service`:

```ts
import {Service} from "encore.dev/service";

export default new Service("my-service");
```

Encore will consider this directory and all its subdirectories as part of the service.

#### 6. Create a catch-all handler for your HTTP router

Now let's mount your existing app router under a [Raw endpoint](/docs/ts/primitives/raw-endpoints), which is an Encore API endpoint type that gives you access to the underlying HTTP request.

Here's a basic code example:

```typescript
import { api, RawRequest, RawResponse } from "encore.dev/api";
import express, { request, response } from "express";

Object.setPrototypeOf(request, RawRequest.prototype);
Object.setPrototypeOf(response, RawResponse.prototype);

const app = express();

app.get('/foo', (req: any, res) => {
  res.send('Hello World!')
})

export const expressApp = api.raw(
  { expose: true, method: "*", path: "/!rest" },
  app,
);
```

By mounting your existing app router in this way, it will work as a catch-all handler for all HTTP requests and responses.

#### 7. Run you app locally

You will now be able to run your Express.js app locally using the `encore run` command.

#### Next steps: Incrementally move over Encore.ts to get all the benefits

You can now gradually break out specific endpoints using the Encore's [API declarations](#apis) and introduce infrastructure declarations for databases and cron jobs etc. This will let Encore.ts understand your application and unlock all Encore.ts features. See the [Feature-by-feature migration](#feature-by-feature-migration) section for more details. You will eventually be able to remove Express.js as a dependency and run your app entirely on Encore.ts.

You can also [join Discord](https://encore.dev/discord) to ask questions and meet fellow Encore developers.

#### Forklift example

<div className="not-prose my-10">
   <Editor projectName="expressForklift" />
</div>


</Accordion>


<Accordion>

### Full migration

This approach aims to fully replace your applications dependency on Express.js with Encore.ts, unlocking all the features and performance of Encore.ts.

Below are two examples that you can use to identify the refactoring you will need to do. In the next section you will find a [Feature-by-feature migration](#feature-by-feature-migration) guide to help you with the refactoring details.

**Approach benefits**

- Get all the advantages of Encore.ts, like [distributed tracing](/docs/ts/observability/tracing) and [architecture diagrams](/docs/ts/observability/flow), which rely on the [Encore application model](/docs/ts/concepts/application-model).
- Get the [full performance benefit](https://encore.dev/blog/event-loops) of Encore.ts - **9x faster** than Express.js.

**Approach drawbacks**

- This approach may require more time and effort up front compared to the [Incremental migration strategy](#incremental-migration-strategy).

#### App comparison

Here is a side-by-side comparison of an Express.js app and an Encore.ts app. The examples show how to create APIs, handle request validation, error handling, serving static files, and rendering templates.

**Express.js**

<div className="not-prose my-10">
   <Editor projectName="expressVsEncore" />
</div>

**Encore.ts**

<div className="not-prose my-10">
   <Editor projectName="encoreVsExpress" />
</div>

</Accordion>

## Feature-by-feature migration

Check out our [Express.js compared to Encore.ts example](https://github.com/encoredev/examples/tree/main/ts/expressjs-migration) on GitHub for all of the code snippets in this feature comparison.

<Accordion>

### APIs

With Express.js, you create APIs using the `app.get`, `app.post`, `app.put`, `app.delete` functions. These functions
take a path and a callback function. You then use the `req` and `res` objects to handle the request and response.

With Encore.ts, you create APIs using the `api` function. This function takes an options object and a callback function.
The main difference compared to Express.js is that Encore.ts is type-safe, meaning you define the request and response
schemas in the callback function. You then return an object matching the response schema. In case you need to operate at
a lower abstraction level, Encore supports defining raw endpoints that let you access the underlying HTTP request.
Learn more in the [API Schemas docs](/docs/ts/primitives/defining-apis#api-schemas).

**Express.js**

```typescript
import express, {Request, Response} from "express";

const app: Express = express();

// GET request with dynamic path parameter
app.get("/hello/:name", (req: Request, res: Response) => {
  const msg = `Hello ${req.params.name}!`;
  res.json({message: msg});
})

// GET request with query string parameter
app.get("/hello", (req: Request, res: Response) => {
  const msg = `Hello ${req.query.name}!`;
  res.json({message: msg});
});

// POST request example with JSON body
app.post("/order", (req: Request, res: Response) => {
  const price = req.body.price;
  const orderId = req.body.orderId;
  // Handle order logic
  res.json({message: "Order has been placed"});
});
```

**Encore.ts**

```typescript
import {api, Query} from "encore.dev/api";

// Dynamic path parameter :name
export const dynamicPathParamExample = api(
  {expose: true, method: "GET", path: "/hello/:name"},
  async ({name}: { name: string }): Promise<{ message: string }> => {
    const msg = `Hello ${name}!`;
    return {message: msg};
  },
);

interface RequestParams {
  // Encore will now automatically parse the query string parameter
  name?: Query<string>;
}

// Query string parameter ?name
export const queryStringExample = api(
  {expose: true, method: "GET", path: "/hello"},
  async ({name}: RequestParams): Promise<{ message: string }> => {
    const msg = `Hello ${name}!`;
    return {message: msg};
  },
);

interface OrderRequest {
  price: string;
  orderId: number;
}

// POST request example with JSON body
export const order = api(
  {expose: true, method: "POST", path: "/order"},
  async ({price, orderId}: OrderRequest): Promise<{ message: string }> => {
    // Handle order logic
    console.log(price, orderId)

    return {message: "Order has been placed"};
  },
);

// Raw endpoint
export const myRawEndpoint = api.raw(
  {expose: true, path: "/raw", method: "GET"},
  async (req, resp) => {
    resp.writeHead(200, {"Content-Type": "text/plain"});
    resp.end("Hello, raw world!");
  },
);
```

</Accordion>

<Accordion>

### Microservices communication

Express.js does not have built-in support for creating microservices or for service-to-service communication. You will most likely use
`fetch` or something equivalent to call another service.

With Encore.ts, calling another service is just like calling a local function, with complete type-safety. Under the hood, Encore.ts will translate this function call into an actual service-to-service HTTP call, resulting in trace data being generated for each call.
Learn more in our [Service-to-Service Communication docs](/docs/ts/primitives/app-structure#multi-service-application-distributed-system).

**Express.js**

```typescript
import express, {Request, Response} from "express";

const app: Express = express();

app.get("/save-post", async (req: Request, res: Response) => {
  try {
    // Calling another service using fetch
    const resp = await fetch("https://another-service/posts", {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        title: req.query.title,
        content: req.query.content,
      }),
    });
    res.json(await resp.json());
  } catch (e) {
    res.status(500).json({error: "Could not save post"});
  }
});
```

**Encore.ts**

```typescript
import {api} from "encore.dev/api";
import {anotherService} from "~encore/clients";

export const microserviceCommunication = api(
  {expose: true, method: "GET", path: "/call"},
  async (): Promise<{ message: string }> => {
    // Calling the foo endpoint in anotherService
    const fooResponse = await anotherService.foo();

    const msg = `Data from another service ${fooResponse.data}!`;
    return {message: msg};
  },
);

```

</Accordion>


<Accordion>

### Authentication

In Express.js you can create a middleware function that checks if the user is authenticated. You can then use this
middleware function in your routes to protect them. You will have to specify the middleware function for each route that
requires authentication.

With Encore.ts, when an API is defined with `auth: true`, you must define an authentication handler in your application.
The authentication handler is responsible for inspecting incoming requests to determine what user is authenticated.

The authentication handler is defined similarly to API endpoints, using the `authHandler` function imported from
`encore.dev/auth`. Like API endpoints, the authentication handler defines what request information it's interested in,
in the form of HTTP headers, query strings, or cookies.

If a request has been successfully authenticated, the API Gateway forwards the authentication data to the target
endpoint. The endpoint can query the available auth data from the `getAuthData` function, available from the
`~encore/auth`
module.

Learn more in our [Auth Handler docs](/docs/ts/develop/auth)

**Express.js**

```typescript
import express, {NextFunction, Request, Response} from "express";

const app: Express = express();

// Auth middleware
function authMiddleware(req: Request, res: Response, next: NextFunction) {
  // TODO: Validate up auth token and verify that this is an authenticated user
  const isInvalidUser = req.headers["authorization"] === undefined;

  if (isInvalidUser) {
    res.status(401).json({error: "invalid request"});
  } else {
    next();
  }
}

// Endpoint that requires auth
app.get("/dashboard", authMiddleware, (_, res: Response) => {
  res.json({message: "Secret dashboard message"});
});
```

**Encore.ts**

```typescript
import { api, APIError, Gateway, Header } from "encore.dev/api";
import { authHandler } from "encore.dev/auth";
import { getAuthData } from "~encore/auth";

interface AuthParams {
  authorization: Header<"Authorization">;
}

// The function passed to authHandler will be called for all incoming API call that requires authentication.
export const myAuthHandler = authHandler(
  async (params: AuthParams): Promise<{ userID: string }> => {
    // TODO: Validate up auth token and verify that this is an authenticated user
    const isInvalidUser = params.authorization === undefined;

    if (isInvalidUser) {
      throw APIError.unauthenticated("Invalid user ID");
    }

    return { userID: "user123" };
  },
);

export const gateway = new Gateway({ authHandler: myAuthHandler });

// Auth endpoint example
export const dashboardEndpoint = api(
  // Setting auth to true will require the user to be authenticated
  { auth: true, method: "GET", path: "/dashboard" },
  async (): Promise<{ message: string; userID: string }> => {
    return {
      message: "Secret dashboard message",
      userID: getAuthData()!.userID,
    };
  },
);
```

</Accordion>

<Accordion>

### Request validation

Express.js does not have built-in request validation. You have to use a library
like [Zod](https://github.com/colinhacks/zod).

With Encore.ts, request validation for headers, query params and body is. You supply a schema for the request object and
in the request payload does not match the schema the API will return a 400 error.
Learn more in the [API Schemas docs](/docs/ts/primitives/defining-apis#api-schemas).

**Express.js**

```typescript
import express, {NextFunction, Request, Response} from "express";
import {z, ZodError} from "zod";

const app: Express = express();

// Request validation middleware
function validateData(schemas: {
  body: z.ZodObject<any, any>;
  query: z.ZodObject<any, any>;
  headers: z.ZodObject<any, any>;
}) {
  return (req: Request, res: Response, next: NextFunction) => {
    try {
      // Validate headers
      schemas.headers.parse(req.headers);

      // Validate request body
      schemas.body.parse(req.body);

      // Validate query params
      schemas.query.parse(req.query);

      next();
    } catch (error) {
      if (error instanceof ZodError) {
        const errorMessages = error.errors.map((issue: any) => ({
          message: `${issue.path.join(".")} is ${issue.message}`,
        }));
        res.status(400).json({error: "Invalid data", details: errorMessages});
      } else {
        res.status(500).json({error: "Internal Server Error"});
      }
    }
  };
}

// Request body validation schemas
const bodySchema = z.object({
  someKey: z.string().optional(),
  someOtherKey: z.number().optional(),
  requiredKey: z.array(z.number()),
  nullableKey: z.number().nullable().optional(),
  multipleTypesKey: z.union([z.boolean(), z.number()]).optional(),
  enumKey: z.enum(["John", "Foo"]).optional(),
});

// Query string validation schemas
const queryStringSchema = z.object({
  name: z.string().optional(),
});

// Headers validation schemas
const headersSchema = z.object({
  "x-foo": z.string().optional(),
});

// Request validation example using Zod
app.post(
  "/validate",
  validateData({
    headers: headersSchema,
    body: bodySchema,
    query: queryStringSchema,
  }),
  (_: Request, res: Response) => {
    res.json({message: "Validation succeeded"});
  },
);
```

**Encore.ts**

```typescript
import {api, Header, Query} from "encore.dev/api";

enum EnumType {
  FOO = "foo",
  BAR = "bar",
}

// Encore.ts automatically validates the request schema and returns and error
// if the request does not match the schema.
interface RequestSchema {
  foo: Header<"x-foo">;
  name?: Query<string>;

  someKey?: string;
  someOtherKey?: number;
  requiredKey: number[];
  nullableKey?: number | null;
  multipleTypesKey?: boolean | number;
  enumKey?: EnumType;
}

// Validate a request
export const schema = api(
  {expose: true, method: "POST", path: "/validate"},
  (data: RequestSchema): { message: string } => {
    console.log(data);
    return {message: "Validation succeeded"};
  },
);
```

</Accordion>

<Accordion>

### Error handling

In Express.js you either throw an error (which results in a 500 response) or use the `status` function to set the status
code of the response.

In Encore.ts throwing an error will result in a 500 response. You can also use the `APIError` class to return specific
error codes. Learn more in our [API Errors docs](/docs/ts/primitives/errors).

**Express.js**

```typescript
import express, {Request, Response} from "express";

const app: Express = express();

// Default error handler
app.get("/broken", (req, res) => {
  throw new Error("BROKEN"); // This will result in a 500 error
});

// Returning specific error code
app.get("/get-user", (req: Request, res: Response) => {
  const id = req.query.id || "";
  if (id.length !== 3) {
    res.status(400).json({error: "invalid id format"});
  }
  // TODO: Fetch something from the DB
  res.json({user: "Simon"});
});
```

**Encore.ts**

```typescript
import {api, APIError} from "encore.dev/api"; // Default error handler

// Default error handler
export const broken = api(
  {expose: true, method: "GET", path: "/broken"},
  async (): Promise<void> => {
    throw new Error("This is a broken endpoint"); // This will result in a 500 error
  },
);

// Returning specific error code
export const brokenWithErrorCode = api(
  {expose: true, method: "GET", path: "/broken/:id"},
  async ({id}: { id: string }): Promise<{ user: string }> => {
    if (id.length !== 3) {
      throw APIError.invalidArgument("invalid id format");
    }
    // TODO: Fetch something from the DB
    return {user: "Simon"};
  },
);
```

</Accordion>

<Accordion>

### Serving static files

Express.js has a built-in middleware function to serve static files. You can use the `express.static` function to serve
files from a specific directory.

Encore.ts also has built-in support for static file serving with the
`api.static` method. The files are served directly from the Encore.ts Rust Runtime. This means that zero JavaScript code is executed to serve the files, freeing up the Node.js runtime to focus on executing business logic. This dramatically speeds up both the static file serving, as well as improving the latency of your API endpoints. Learn more in our [Static Assets docs](/docs/ts/primitives/static-assets).

**Express.js**

```typescript
import express from "express";

const app: Express = express();

app.use("/assets", express.static("assets"));
```

**Encore.ts**

```typescript
import { api } from "encore.dev/api";

export const assets = api.static(
  { expose: true, path: "/assets/*path", dir: "./assets" },
);
```

</Accordion>

<Accordion>

### Template rendering

Express.js has a built-in support for rendering templates.

With Encore.ts you can use the `api.raw` function to serve HTML templates, in this example we are using Handlebars.js
but you can use whichever templating engine you prefer. Learn more in
our [Raw Endpoints docs](/docs/ts/primitives/raw-endpoints)

**Express.js**

```typescript
import express, {Request, Response} from "express";

const app: Express = express();

app.set("view engine", "pug"); // Set view engine to Pug

// Template engine example. This will render the index.pug file in the views directory
app.get("/html", (_, res) => {
  res.render("index", {title: "Hey", message: "Hello there!"});
});
```

**Encore.ts**

```typescript
import {api} from "encore.dev/api";
import Handlebars from "handlebars";

const html = `
<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8"/>
  <link rel="stylesheet" href="/assets/styles.css">
</head>
<body>
<h1>Hello {{name}}!</h1>
</body>
</html>
`;

// Making use of raw endpoints to serve dynamic templates.
// https://encore.dev/docs/ts/primitives/raw-endpoints
export const serveHTML = api.raw(
  {expose: true, path: "/html", method: "GET"},
  async (req, resp) => {
    const template = Handlebars.compile(html);

    resp.setHeader("Content-Type", "text/html");
    resp.end(template({name: "Simon"}));
  },
);
```

</Accordion>

<Accordion>

### Testing

Express.js does not have built-in testing support. You can use libraries like [Vitest](https://vitest.dev/) and
[Supertest](https://www.npmjs.com/package/supertest).

With Encore.ts you are able to call the API endpoints directly in your tests, just like any other function. You then run
the tests using the `encore test` command. Learn more in our [Testing docs](/docs/ts/develop/testing).

**Express.js**

```typescript
import {describe, expect, test} from "vitest";
import request from "supertest";
import express from "express";
import getRequestExample from "../get-request-example";

/**
 * We need to add the supertest library to make fake HTTP requests to the Express.js app without having to
 * start the server. We also use the vitest library to write tests.
 */
describe("Express App", () => {
  const app = express();
  app.use("/", getRequestExample);

  test("should respond with a greeting message", async () => {
    const response = await request(app).get("/hello/John");
    expect(response.status).to.equal(200);
    expect(response.body).to.have.property("message");
    expect(response.body.message).to.equal("Hello John!");
  });
});
```

**Encore.ts**

```typescript
import {describe, expect, test} from "vitest";
import {dynamicPathParamExample} from "../get-request-example";

// This test suite demonstrates how to test an Encore route.
// Run tests using the `encore test` command.
describe("Encore app", () => {
  test("should respond with a greeting message", async () => {
    // You can call the Encore.ts endpoint directly in your tests,
    // just like any other function.
    const resp = await dynamicPathParamExample({name: "world"});
    expect(resp.message).toBe("Hello world!");
  });
});

```

</Accordion>

<Accordion>

### Database

Express.js does not have built-in database support. You can use libraries like [pg-promise](https://www.npmjs.com/package/pg-promise) to connect to a
PostgreSQL database but you also have to manage Docker Compose files for different environments.

With Encore.ts, you create a database by importing `encore.dev/storage/sqldb` and calling
`new SQLDatabase`, assigning the result to a top-level variable.

Database schemas are defined by creating migration files. Each migration runs in order and expresses the change in the database schema from the previous migration.

Encore.ts automatically provisions databases to match what your application requires. Encore.ts provisions databases in an appropriate way depending on the environment. When running locally, Encore creates a database cluster using Docker. In the cloud, it depends on the environment type:

To query data, use the `.query` or
`.queryRow` methods. To insert data, or to make database queryies that don't return any rows, use `.exec`.

Learn more in our [Database docs](/docs/ts/primitives/databases).

**Express.js**

```typescript
-- db.ts --
import express, {Request, Response} from "express";
import pgPromise from "pg-promise";

const app: Express = express();

// Connect to the DB with the credentials from docker-compose.yml
const db = pgPromise()({
  host: "localhost",
  port: 5432,
  database: "database",
  user: "user1",
  password: "user1@123",
});

interface User {
  name: string;
  id: number;
}

// Get one User from DB
app.get("/user/:id", async (req: Request, res: Response) => {
  const user = await db.oneOrNone<User>(
    `
        SELECT *
        FROM users
        WHERE id = $1
    `,
    req.params.id,
  );

  res.json({user});
});
-- docker-compose.yml --
version: '3.8'

services:
  db:
    build:
      context: .
      dockerfile: Dockerfile.postgis  # Use custom Dockerfile
    restart: always
    environment:
      POSTGRES_USER: user1
      POSTGRES_PASSWORD: user1@123
      POSTGRES_DB: database
    healthcheck:
      # This command checks if the database is ready, right on the source db server
      test: [ "CMD-SHELL", "pg_isready" ]
      interval: 5s
      timeout: 5s
      retries: 5
    ports:
      - "5432:5432"
    volumes:
      - postgres_data_v:/var/lib/postgresql/data
volumes:
  postgres_data_v:
-- Dockerfile.postgis --
FROM postgres:latest

# Install PostGIS extension
RUN apt-get update \
    && apt-get install -y postgis postgresql-12-postgis-3 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# To execute some initial queries, we can write queries in init.sql
COPY init.sql /docker-entrypoint-initdb.d/

# Enable PostGIS extension
RUN echo "CREATE EXTENSION IF NOT EXISTS postgis;" >> /docker-entrypoint-initdb.d/init.sqld
```

**Encore.ts**

```typescript
-- db.ts --
import {api} from "encore.dev/api";
import {SQLDatabase} from "encore.dev/storage/sqldb";

// Define a database named 'users', using the database migrations in the "./migrations" folder.
// Encore automatically provisions, migrates, and connects to the database.
export const DB = new SQLDatabase("users", {
  migrations: "./migrations",
});

interface User {
  name: string;
  id: number;
}

// Get one User from DB
export const getUser = api(
  {expose: true, method: "GET", path: "/user/:id"},
  async ({id}: { id: number }): Promise<{ user: User | null }> => {
    const user = await DB.queryRow<User>`
        SELECT name
        FROM users
        WHERE id = ${id}
    `;

    return {user};
  },
);

// Add User from DB
export const addUser = api(
  { expose: true, method: "POST", path: "/user" },
  async ({ name }: { name: string }): Promise<void> => {
    await DB.exec`
        INSERT INTO users (name)
        VALUES (${name})
    `;
    return;
  },
);
-- migrations/1_create_tables.up.sql --
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
```

</Accordion>

<Accordion>

### Logging

Express.js does not have built-in support for logging. You can use libraries like [Winston](https://www.npmjs.com/package/winston) to log messages.

Encore.ts offers built-in support for Structured Logging, which combines a free-form log message with structured and type-safe key-value pairs. Logging is integrated with the built-in [Distributed Tracing](/docs/ts/observability/tracing) functionality, and all logs are automatically included in the active trace.

**Encore.ts**

```typescript
import log from "encore.dev/log";

log.error(err, "something went terribly wrong!");
log.info("log message", {is_subscriber: true});
```

</Accordion>

