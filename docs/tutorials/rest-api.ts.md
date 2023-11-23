---
seotitle: How to build a REST API
seodesc: Learn how to build and ship a REST API in just a few minutes, using Typescript and Encore.
title: Building a REST API
subtitle: Learn how to build a URL shortener with a REST API and SQL database
---

In this tutorial you will create a REST API for a URL Shortener service. In a few short minutes, you'll learn how to:

* Create REST APIs with Encore
* Use PostgreSQL databases

This is the end result:
<div className="not-prose mb-10">
   <Editor projectName="urlShortenerTS" />
</div>

<Callout type="info">

To make it easier to follow along, we've laid out a trail of croissants to guide your way.
Whenever you see a 🥐 it means there's something for you to do.

</Callout>

## 1. Create a service and endpoint

Create a new application by running `encore-beta app create --example=empty-ts url-shortener`.

Now let's create a new `url` service.

🥐 In your application's root folder, create a new folder `url` and create a new file `url.ts` that looks like this:

```ts
import { api } from "encore.dev/api";
import { randomBytes } from "node:crypto";

interface URL {
  id: string; // short-form URL id
  url: string; // complete URL, in long form
}

interface ShortenParams {
  url: string; // the URL to shorten
}

// Shorten shortens a URL.
export const shorten = api(
  { method: "POST", path: "/url" },
  async ({ url }: ShortenParams): Promise<URL> => {
    const id = randomBytes(6).toString("base64url");
    return { id, url };
  },
);
```

This sets up the `POST /url` endpoint.

🥐 Let’s see if it works! Start your app by running `encore-beta run`.

You should see this:

```bash
Encore development server running!

Your API is running at:     http://127.0.0.1:4000
Development Dashboard URL:  http://localhost:9400/5g288
3:50PM INF registered API endpoint endpoint=shorten path=/url service=url
```

🥐 Next, call your endpoint:

```shell
$ curl http://localhost:4000/url -d '{"url": "https://encore.dev"}'
```

You should see this:

```bash
{
  "id": "5cJpBVRp",
  "url": "https://encore.dev"
}
```

It works! There’s just one problem...

Right now, we’re not actually storing the URL anywhere. That means we can generate shortened IDs but there’s no way to get back to the original URL! We need to store a mapping from the short ID to the complete URL.

## 2. Save URLs in a database
Fortunately, Encore makes it really easy to set up a PostgreSQL database to store our data. To do so, we first define a **database schema**, in the form of a migration file.

🥐 Create a new folder named `migrations` inside the `url` folder. Then, inside the `migrations` folder, create an initial database migration file named `1_create_tables.up.sql`. The file name format is important (it must start with `1_` and end in `.up.sql`).

🥐 Add the following contents to the file:

```sql
CREATE TABLE url (
	id TEXT PRIMARY KEY,
	original_url TEXT NOT NULL
);
```

🥐 Next, go back to the `url/url.ts` file and import the `SQLDatabase` class from `encore.dev/storage/sqldb` module by modifying the imports to look like this:

```ts
import { api } from "encore.dev/api";
import { randomBytes } from "node:crypto";
import { SQLDatabase } from "encore.dev/storage/sqldb";
```

🥐 Now, to insert data into our database, let’s create an instance of the `SQLDatabase` class:

```ts
const DB = new SQLDatabase("url", { migrations: "./migrations" });
```

🥐 Lastly, we can update our `shorten` function to insert into the database:

```ts
export const shorten = api(
  { method: "POST", path: "/url" },
  async ({ url }: ShortenParams): Promise<URL> => {
    const id = randomBytes(6).toString("base64url");
    await DB.exec`
      INSERT INTO url (id, original_url)
      VALUES (${id}, ${url})
    `;
    return { id, url };
  },
);
```

<Callout type="important">

Before running your application, make sure you have [Docker](https://www.docker.com) installed and running. It's required to locally run Encore applications with databases.

</Callout>

🥐 Next, start the application again with `encore run` and Encore automatically sets up your database.

(In case your application won't run, check the [databases troubleshooting guide](/docs/develop/databases#troubleshooting).)

🥐 Now let's call the API again:

```shell
$ curl http://localhost:4000/url -d '{"url": "https://encore.dev"}'
```

🥐 Finally, let's verify that it was saved in the database by running  `encore db shell url` from the app root directory:

```shell
$ encore db shell url
psql (13.1, server 11.12)
Type "help" for help.

url=# select * from url;
    id    |    original_url
----------+--------------------
 zr6RmZc4 | https://encore.dev
(1 row)
```

That was easy!

## 3. Add endpoint to retrieve URLs
To complete our URL shortener API, let’s add the endpoint to retrieve a URL given its short id.

🥐 Add this endpoint to `url/url.ts`:

```ts
export const get = api(
  { method: "GET", path: "/url/:id" },
  async ({ id }: { id: string }): Promise<URL> => {
    const row = await DB.queryRow`
      SELECT original_url FROM url WHERE id = ${id}
    `;
    if (!row) throw new Error("url not found");
    return { id, url: row.original_url };
  },
);
```

Encore uses the `/url/:id` syntax to represent a path with a parameter. The `id` name corresponds to the parameter name in the function signature. In this case it is of type `string`, but you can also use other built-in types like `number` or `boolean` if you want to restrict the values.

🥐 Let’s make sure it works by calling it:

```shell
$ curl http://localhost:4000/url/zr6RmZc4
```

You should now see this:

```bash
{
  "id": "zr6RmZc4",
  "url": "https://encore.dev"
}
```

And there you have it! That's how you build REST APIs in Encore.

## What's next

Now that you know how to build a backend with a database, you're ready to let your creativity flow and begin building your next great idea!

We're excited to hear what you're going to build with Encore, join the pioneering developer community on [Slack](/slack) and share your story.

