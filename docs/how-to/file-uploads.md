---
seotitle: How to handle file uploads in you Encore.ts application
seodesc: Learn how to store file uploads as bytes in a database and serving them back to the client.
title: Handling file uploads
lang: ts
---

In this guide you will learn how to handle file uploads from a client in your Encore.ts backend.

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/ts/file-upload" 
    desc="Handling file uploads and storing file data in a database" 
/>

## Storing a single file in a database

Breakdown of the example:
* We have a [PostgreSQL database](/docs/ts/primitives/databases) table named `files` with columns `name` and `data` to store the file name and the file data.
* We have a [Raw Endpoint](/docs/ts/primitives/raw-endpoints) to handle file uploads. The endpoint has a `bodyLimit` set to `null` to allow for unlimited file size. 
* We make use of the [busboy](https://www.npmjs.com/package/busboy) library to help with the file handling.
* We convert the file data to a `Buffer` and store the file as a `BYTEA` in the database.

```ts
-- upload.ts --
import { api } from "encore.dev/api";
import log from "encore.dev/log";
import busboy from "busboy";
import { SQLDatabase } from "encore.dev/storage/sqldb";

// Define a database named 'files', using the database migrations
// in the "./migrations" folder. Encore automatically provisions,
// migrates, and connects to the database.
export const DB = new SQLDatabase("files", {
  migrations: "./migrations",
});

type FileEntry = { data: any[]; filename: string };

/**
 * Raw endpoint for storing a single file to the database.
 * Setting bodyLimit to null allows for unlimited file size.
 */
export const save = api.raw(
  { expose: true, method: "POST", path: "/upload", bodyLimit: null },
  async (req, res) => {
    const bb = busboy({
      headers: req.headers,
      limits: { files: 1 },
    });
    const entry: FileEntry = { filename: "", data: [] };

    bb.on("file", (_, file, info) => {
      entry.filename = info.filename;
      file
        .on("data", (data) => {
          entry.data.push(data);
        })
        .on("close", () => {
          log.info(`File ${entry.filename} uploaded`);
        })
        .on("error", (err) => {
          bb.emit("error", err);
        });
    });

    bb.on("close", async () => {
      try {
        const buf = Buffer.concat(entry.data);
        await DB.exec`
            INSERT INTO files (name, data)
            VALUES (${entry.filename}, ${buf})
            ON CONFLICT (name) DO UPDATE
                SET data = ${buf}
        `;
        log.info(`File ${entry.filename} saved`);

        // Redirect to the root page
        res.writeHead(303, { Connection: "close", Location: "/" });
        res.end();
      } catch (err) {
        bb.emit("error", err);
      }
    });

    bb.on("error", async (err) => {
      res.writeHead(500, { Connection: "close" });
      res.end(`Error: ${(err as Error).message}`);
    });

    req.pipe(bb);
    return;
  },
);
-- migrations/1_create_tables.up.sql --
CREATE TABLE files (
    name TEXT PRIMARY KEY,
    data BYTEA NOT NULL
);
```

### Frontend

```html
<form method="POST" enctype="multipart/form-data" action="/upload">
    <label for="filefield">Single file upload:</label><br>
    <input type="file" name="filefield">
    <input type="submit">
</form>
```

## Handling multiple file uploads

When handling multiple file uploads, we can use the same approach as above, but we need to handle multiple files in the busboy event listeners. When storing the files in the database, we loop through the files and save them one by one.

```ts
export const saveMultiple = api.raw(
  { expose: true, method: "POST", path: "/upload-multiple", bodyLimit: null },
  async (req, res) => {
    const bb = busboy({ headers: req.headers });
    const entries: FileEntry[] = [];

    bb.on("file", (_, file, info) => {
      const entry: FileEntry = { filename: info.filename, data: [] };

      file
        .on("data", (data) => {
          entry.data.push(data);
        })
        .on("close", () => {
          entries.push(entry);
        })
        .on("error", (err) => {
          bb.emit("error", err);
        });
    });

    bb.on("close", async () => {
      try {
        for (const entry of entries) {
          const buf = Buffer.concat(entry.data);
          await DB.exec`
              INSERT INTO files (name, data)
              VALUES (${entry.filename}, ${buf})
              ON CONFLICT (name) DO UPDATE
                  SET data = ${buf}
          `;
          log.info(`File ${entry.filename} saved`);
        }

        // Redirect to the root page
        res.writeHead(303, { Connection: "close", Location: "/" });
        res.end();
      } catch (err) {
        bb.emit("error", err);
      }
    });

    bb.on("error", async (err) => {
      res.writeHead(500, { Connection: "close" });
      res.end(`Error: ${(err as Error).message}`);
    });

    req.pipe(bb);
    return;
  },
);
```

### Frontend

```html
<form method="POST" enctype="multipart/form-data" action="/upload-multiple">
    <label for="filefield">Multiple files upload:</label><br>
    <input type="file" name="filefield" multiple>
    <input type="submit">
</form>
```

## Handling large files

In order to not run into a **Maximum request length exceeded**-error when uploading large files you might need to adjust the endpoints `bodyLimit`. You can also set the `bodyLimit` to `null` to allow for unlimited file size uploads. If unset it defaults to 2MiB.

## Retrieving files from the database

When retrieving files from the database, we can use a GET endpoint to fetch the file data by its name. We can then serve the file back to the client by creating a `Buffer` from the file data and sending it in the response. 

```ts
import { api } from "encore.dev/api";
import { APICallMeta, currentRequest } from "encore.dev"; 

export const DB = new SQLDatabase("files", {
  migrations: "./migrations",
});

export const get = api.raw(
  { expose: true, method: "GET", path: "/files/:name" },
  async (req, resp) => {
    try {
      const { name } = (currentRequest() as APICallMeta).pathParams;
      const row = await DB.queryRow`
          SELECT data
          FROM files
          WHERE name = ${name}`;
      if (!row) {
        resp.writeHead(404);
        resp.end("File not found");
        return;
      }

      const chunk = Buffer.from(row.data);
      resp.writeHead(200, { Connection: "close" });
      resp.end(chunk);
    } catch (err) {
      resp.writeHead(500);
      resp.end((err as Error).message);
    }
  },
);
```

You should now be able to retrieve a file from the database by making a GET request to `http://localhost:4000/files/name-of-file.ext`. 
