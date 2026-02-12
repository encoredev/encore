---
seotitle: Using Better Auth with Encore.ts for Authentication
seodesc: Learn how to add production-ready authentication to your Encore.ts application using Better Auth, with automatic database provisioning and secrets management.
title: Better Auth
lang: ts
---

[Better Auth](https://www.better-auth.com) is a comprehensive TypeScript authentication library that supports email/password, OAuth, two-factor, magic links, and sessions. Combined with Encore's automatic [database provisioning](https://encore.dev/docs/ts/primitives/databases) and [secrets management](https://encore.dev/docs/ts/primitives/secrets), you get production-ready auth without managing any infrastructure.

To get started quickly, create a new app from the example:

```shell
$ encore app create --example=ts/betterauth
```

Or follow the steps below to add Better Auth to an existing Encore app.

<Callout type="info">

If you haven't installed Encore yet, see the [installation guide](https://encore.dev/docs/ts/install) first.

</Callout>

## Install

```shell
$ npm install better-auth pg
```

## Set up the database

Better Auth needs a database for users and sessions. Encore [provisions and manages databases](https://encore.dev/docs/ts/primitives/databases) for you automatically — just define it in code:

```ts
-- db.ts --
import { SQLDatabase } from "encore.dev/storage/sqldb";

export const db = new SQLDatabase("auth", {
  migrations: "./migrations",
});
```

<Callout type="info">

Locally, Encore starts a PostgreSQL instance automatically when you run `encore run`. You'll need [Docker](https://docker.com/get-started/) running for the local database.

</Callout>

## Configure Better Auth

Create the Better Auth instance using Encore's database and secrets:

```ts
-- auth.ts --
import { betterAuth } from "better-auth";
import { Pool } from "pg";
import { secret } from "encore.dev/config";
import { db } from "./db";

const authSecret = secret("AuthSecret");

const pool = new Pool({
  connectionString: db.connectionString,
});

export const auth = betterAuth({
  secret: authSecret(),
  basePath: "/auth",
  database: pool,
  trustedOrigins: ["http://localhost:4000"],
  emailAndPassword: {
    enabled: true,
  },
  socialProviders: {
    github: {
      clientId: secret("GithubClientId")(),
      clientSecret: secret("GithubClientSecret")(),
    },
  },
});
```

Set the secrets using the Encore CLI:

```shell
$ encore secret set --type dev,local,pr,production AuthSecret
$ encore secret set --type dev,local,pr,production GithubClientId
$ encore secret set --type dev,local,pr,production GithubClientSecret
```

<Callout type="info">

**Tip:** Generate a strong auth secret with `openssl rand -base64 32` and paste it when prompted for `AuthSecret`.

</Callout>

<Callout type="info">

Locally, secrets are stored on your machine and injected when you run `encore run`. No `.env` files needed.

</Callout>

## Connect to Encore's auth handler

Wire Better Auth into Encore's [authentication system](https://encore.dev/docs/ts/develop/auth) so you can use `auth: true` on any API endpoint:

```ts
-- handler.ts --
import { APIError, Gateway } from "encore.dev/api";
import { authHandler } from "encore.dev/auth";
import { Header } from "encore.dev/api";
import { auth } from "./auth";

interface AuthParams {
  authorization: Header<"Authorization">;
}

interface AuthData {
  userID: string;
}

const handler = authHandler<AuthParams, AuthData>(
  async (params) => {
    const session = await auth.api.getSession({
      headers: new Headers({
        authorization: params.authorization,
      }),
    });

    if (!session) {
      throw APIError.unauthenticated("invalid session");
    }

    return { userID: session.user.id };
  }
);

export const gateway = new Gateway({ authHandler: handler });
```

## Expose Better Auth routes

Better Auth needs HTTP routes for sign-in, sign-up, and OAuth callbacks. Expose these using a [raw endpoint](https://encore.dev/docs/ts/primitives/raw-endpoints):

```ts
-- routes.ts --
import { api } from "encore.dev/api";
import { auth } from "./auth";

// Better Auth expects a Web Request, but Encore raw endpoints receive
// a Node.js IncomingMessage. We convert between the two formats.
export const authRoutes = api.raw(
  { expose: true, path: "/auth/*path", method: "*" },
  async (req, res) => {
    // Read the request body
    const chunks: Buffer[] = [];
    for await (const chunk of req) {
      chunks.push(chunk);
    }
    const body = Buffer.concat(chunks);

    // Build a Web Request from the Node.js request
    const headers = new Headers();
    for (const [key, value] of Object.entries(req.headers)) {
      if (value) headers.append(key, Array.isArray(value) ? value.join(", ") : value);
    }

    const url = `http://${req.headers.host}${req.url}`;
    const webReq = new Request(url, {
      method: req.method,
      headers,
      body: ["GET", "HEAD"].includes(req.method || "") ? undefined : body,
    });

    // Pass to Better Auth and forward the response
    const response = await auth.handler(webReq);

    response.headers.forEach((value, key) => {
      res.setHeader(key, value);
    });
    res.writeHead(response.status);
    res.end(await response.text());
  }
);
```

## Use in your endpoints

Any endpoint with `auth: true` will now require a valid Better Auth session:

```ts
import { api } from "encore.dev/api";
import { getAuthData } from "~encore/auth";

export const getProfile = api(
  { auth: true, expose: true, method: "GET", path: "/profile" },
  async (): Promise<{ userID: string }> => {
    const data = getAuthData()!;
    return { userID: data.userID };
  }
);
```

## Deploy

When you deploy, Encore automatically provisions and manages the infrastructure your app needs. For Better Auth integrations, this includes:

- **Database** — Cloud SQL on GCP, RDS on AWS. Migrations run automatically on deploy
- **Secrets** — encrypted per environment (preview, staging, production), never shared between them
- **Networking** — TLS, load balancing, DNS

Your application code stays the same regardless of where you deploy.

### Self-hosting

Build a Docker image and deploy anywhere:

```shell
$ encore build docker my-app:latest
```

See [Self-hosting](https://encore.dev/docs/ts/self-host/build) for more details on building and deploying Docker images.

### Encore Cloud

Push your code and Encore handles the rest.

```shell
$ git push encore main
```

Start free on Encore Cloud, then connect your own AWS or GCP account when you're ready. Your application code stays exactly the same — Encore automatically provisions the right infrastructure in your cloud account, so there's nothing to rewrite or migrate. See [Connect your cloud account](https://encore.dev/docs/platform/deploy/own-cloud) for details.

## Related resources

- [Encore authentication docs](https://encore.dev/docs/ts/develop/auth)
- [Better Auth documentation](https://www.better-auth.com/docs)
- [Encore databases](https://encore.dev/docs/ts/primitives/databases)
- [Encore secrets](https://encore.dev/docs/ts/primitives/secrets)
