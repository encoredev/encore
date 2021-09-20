---
title: Building a REST API
---

In this tutorial you will create a REST API for a URL Shortener service.

REST APIs are resource-oriented, meaning that we start by identifying the resources our app needs and designing an URL hierarchy based on them. For our URL Shortener we’ll have just a single resource: the URL.

Our API structure will be:

* `POST /url` — Create a new, shortened URL (returns id)
* `GET /url/:id` — Returns the full URL given an id

Let’s get going!

## Creating our Shorten endpoint
We start by defining a new `url` service. You can use the Encore application you created in the [Quick Start](/docs/quick-start "Quick Start") guide or create a new one from scratch, it’s up to you.

Create a new folder `url` and create a new file `url/url.go` that looks like this:

```go
package url

import (
	"context"
	"crypto/rand"
	"encoding/base64"
)

type URL struct {
	ID  string // short-form URL id
	URL string // complete URL, in long form
}

type ShortenParams struct {
	URL string // the URL to shorten
}

// Shorten shortens a URL.
//encore:api public method=POST path=/url
func Shorten(ctx context.Context, p *ShortenParams) (*URL, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	return &URL{ID: id, URL: p.URL}, nil
}

// generateID generates a random short ID.
func generateID() (string, error) {
	var data [6]byte // 6 bytes of entropy
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data[:]), nil
}
```

This sets up the `POST /url` endpoint (see the `//encore:api` annotation on the `Shorten` function). Let’s see if it works!

```bash
$ encore run
API Base URL:      http://localhost:4000
Dev Dashboard URL: http://localhost:62709/hello-world-cgu2
4:19PM INF registered endpoint path=/url service=url endpoint=Shorten
```

Let’s try calling it:

```bash
$ curl http://localhost:4000/url -d '{"URL": "https://encore.dev"}'
{
  "ID": "5cJpBVRp",
  "URL": "https://encore.dev"
}
```

It works! There’s just one problem... We’re not actually storing the URL anywhere. That means we can generate shortened IDs but there’s no way to get back to the original URL! We need to store a mapping from the short ID to the complete URL.

## Saving URLs in a database
Fortunately, Encore makes it really easy to set up a PostgreSQL database to store our data. To do so, we first define a **database schema**, in the form of a migration file.

Create a new folder named `migrations` inside the `url` folder. Then, create an initial database migration file named `url/migrations/1_create_tables.up.sql`. The file name format is important (it must start with `1_` and end in `.up.sql`).

Add the following contents to the file:

```sql
CREATE TABLE url (
	id TEXT PRIMARY KEY,
	original_url TEXT NOT NULL
);
```

Next, go back to the `url/url.go` file and import the `encore.dev/storage/sqldb` package by modifying the import statement to become:

```go
import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"encore.dev/storage/sqldb"
)
```

Now, to insert data into our database, let’s create a helper function `insert`:

```go
// insert inserts a URL into the database.
func insert(ctx context.Context, id, url string) error {
	_, err := sqldb.Exec(ctx, `
		INSERT INTO url (id, original_url)
		VALUES ($1, $2)
	`, id, url)
	return err
}
```

Finally we can update our `Shorten` function to insert into the database:

```go
func Shorten(ctx context.Context, p *ShortenParams) (*URL, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	} else if err := insert(ctx, id, p.URL); err != nil {
		return nil, err
	}
	return &URL{ID: id, URL: p.URL}, nil
}
```

Start your application again with `encore run` and Encore automatically sets up your database. Let’s try it out!

```bash
$ curl http://localhost:4000/url -d '{"URL": "https://encore.dev"}'
{
  "ID": "zr6RmZc4",
  "URL": "https://encore.dev"
}
```

Let’s verify that it was saved in the database with `encore db shell url`:

```bash
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

## Retrieving the full URL
To complete our URL shortener API, let’s add the endpoint to retrieve a URL given its short id.

Add this endpoint to `url/url.go`:

```go
// Get retrieves the original URL for the id.
//encore:api public method=GET path=/url/:id
func Get(ctx context.Context, id string) (*URL, error) {
	u := &URL{ID: id}
	err := sqldb.QueryRow(ctx, `
		SELECT original_url FROM url
		WHERE id = $1
	`, id).Scan(&u.URL)
	return u, err
}
```

Encore uses the `path=/url/:id` syntax to represent a path with a parameter. The `id` name corresponds to the parameter name in the function signature. In this case it is of type `string`, but you can also use other built-in types like `int` or `bool` if you want to restrict the values.

Let’s make sure it works:

```bash
$ curl http://localhost:4000/url/zr6RmZc4
{
  "ID": "zr6RmZc4",
  "URL": "https://encore.dev"
}
```

And there you have it! That's how you build REST APIs in Encore.