---
seotitle: How to use a template engine in you Encore.ts application
seodesc: Learn how to use a template engine to create server-rendered HTML with dynamic data.
title: Use a template engine
lang: ts
---

In this guide you will learn how to use a template engine, like [EJS](https://ejs.co) and [Handlebars](https://handlebarsjs.com), to create server-rendered HTML views.

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/ts/template-engine" 
    desc="Using EJS as a template engine with Encore.ts" 
/>

## Serving a specific template file

Breakdown of the example:
* We import a NPM package for rendering templates, in this case [EJS](https://ejs.co/).
* We have a [Raw Endpoint](/docs/ts/primitives/raw-endpoints) to handle template rendering, in this case we are serving a specific template file (`person.html`) under `/person`. 
* We make use of the EJS to render the template with the given data.
* We set the `content-type` header to `text-html` and then respond with the generated HTML.

```ts
-- template/template.ts --
import { api } from "encore.dev/api";
import ejs, { Options } from "ejs";

const BASE_PATH = "./template/views";
const ejsOptions: Options = { views: [BASE_PATH] };

export const serveSpecificTemplate = api.raw(
  { expose: true, path: "/person", method: "GET" },
  async (req, resp) => {
    const viewPath = `${BASE_PATH}/person.html`;
    const html = await ejs.renderFile(
      viewPath,
      // Supplying data to the view
      { name: "Simon" },
      ejsOptions,
    );
    resp.setHeader("content-type", "text/html");
    resp.end(html);
  },
);
-- template/views/person.html --
<h1>Person Page</h1>
<p>Name: <%= name %></p>
```

## Serving from a dynamic path

This example is similar to the one above, but in this case we use a fallback path to serve a template file based on the path. We use the `currentRequest` function to get the `path` and then render the template file based on the `path`. If no path is provided, we default to `index.html`.

```ts
import { api } from "encore.dev/api";
import { APICallMeta, currentRequest } from "encore.dev";
import ejs, { Options } from "ejs";

const BASE_PATH = "./template/views";
const ejsOptions: Options = { views: [BASE_PATH] };

export const servePathTemplate = api.raw(
  { expose: true, path: "/!path", method: "GET" },
  async (req, resp) => {
    const { path } = (currentRequest() as APICallMeta).pathParams;
    const viewPath = `${BASE_PATH}/${path ?? "index"}.html`;
    const html = await ejs.renderFile(viewPath, ejsOptions);
    resp.setHeader("content-type", "text/html");
    resp.end(html);
  },
);
```

## Serving inline HTML

In this example we are serving inline HTML with EJS. We use the `ejs.render` function to render the inline HTML with the given data.

```ts
import { api } from "encore.dev/api";
import ejs, { Options } from "ejs";

const inlineHTML = `
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="stylesheet" href="/public/styles.css" >
  </head>
  <body>
    <h1>Static Inline HTML Example</h1>
    <h1>Name: <%= name %>!</h1>
  </body>
</html>
`;

export const serveInlineHTML = api.raw(
  { expose: true, path: "/html", method: "GET" },
  async (req, resp) => {
    const html = ejs.render(inlineHTML, { name: "Simon" });
    resp.setHeader("Content-Type", "text/html");
    resp.end(html);
  },
);
```

## Static files

In the above example we are fetching a stylesheet from the `/public` path. We can use the `api.static` function to serve all files in the `./assets` directory under the `/public` path prefix:

```ts
// Serve all files in the ./assets directory under the /public path prefix.
export const assets = api.static({
  expose: true,
  path: "/public/*path",
  dir: "./assets",
});
```

Learn more about serving static files in the [Static Files](/docs/ts/primitives/static-assets) guide.
