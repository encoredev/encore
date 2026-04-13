---
seotitle: Encore Toolbar – Inspect API requests from your frontend
seodesc: Learn how to add the Encore Toolbar to your frontend to inspect API requests, link to distributed traces, and view backend logs during development.
title: Encore Toolbar
subtitle: Inspect API requests and traces from your frontend
lang: ts
---

The Encore Toolbar is a lightweight, drop-in script that adds a floating developer panel to your frontend application. It automatically intercepts all `fetch()` and `XMLHttpRequest` calls, captures trace IDs from Encore's response headers, and lets you jump directly to the corresponding trace in the [Development Dashboard](/docs/ts/observability/dev-dash) or [Encore Cloud](https://app.encore.cloud).

This is useful when you're building a frontend that talks to an Encore backend and want visibility into what's happening on the backend without switching to a separate tool.

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="https://encore.dev/assets/videos/encore-toolbar.mp4" className="w-full h-full" type="video/mp4" />
</video>

## Installation

Add a single `<script>` tag to your HTML. The toolbar will automatically initialize and begin intercepting requests.

```html
<script src="https://encore.dev/encore-toolbar.js"></script>
```

The toolbar automatically detects your App ID and environment by calling the `/__encore/healthz` endpoint on each request's origin. If auto-detection doesn't work (for example, if have a proxy between your frontend and backend and `/__encore/healthz` is not exposed), you can pass them explicitly:

```html
<script src="https://encore.dev/encore-toolbar.js?appId=my-app&envName=staging"></script>
```

| Parameter | Description |
| --------- | ----------- |
| `appId` | Your Encore app slug (the one in your `app.encore.cloud` URL). |
| `envName` | The environment name, e.g. `staging`, or `production`. |

You can also set or change these values later in the toolbar's Settings panel.

<Callout type="info">

The script patches `window.fetch` and `XMLHttpRequest` at parse time. It must be loaded **before** your application code and **without** `async` or `defer` attributes. Some HTTP libraries (like Axios) store a reference to `fetch` when they initialize, so if the library loads before the toolbar script, those requests won't be intercepted. Place the script tag as early as possible in your `<head>`.

</Callout>

## How it works

When your frontend makes a request to an Encore backend, the backend includes an `x-encore-trace-id` header in the response. The toolbar reads this header and records the request along with its trace ID. Only requests that include this header appear in the toolbar.

For each captured request, the toolbar shows:

- **Method and URL** - the HTTP method and full URL of the request.
- **Status code** - the response status.
- **Request and response bodies** - captured automatically.
- **Query parameters and cookies** - parsed from the request URL and `document.cookie`.
- **Trace link** - a direct link to the trace in the Development Dashboard (local) or Encore Cloud (deployed environments).
- **Backend logs** - when running locally, the toolbar connects to the local Encore daemon and displays backend log output for the selected trace.

## Troubleshooting

### Requests are not being intercepted

The toolbar only captures requests that return an `x-encore-trace-id` response header. If your requests aren't showing up:

1. **Verify the header is present.** Open your browser's Network tab, select a request to your Encore backend, and check the response headers for `x-encore-trace-id`. If the header is missing, the request is not going through Encore's request handling (for example, it might be hitting a non-Encore server or a reverse proxy that strips headers).

2. **Make sure the script loads before your app.** The toolbar patches `fetch` and `XMLHttpRequest` at parse time. If your application code runs before the script is loaded, those early requests won't be captured. Move the `<script>` tag above your application bundle.

3. **Check for script errors.** Open the browser console and look for errors related to the toolbar script. A network failure loading the script (e.g. a Content Security Policy blocking it) will prevent it from initializing.

### Trace linking is not working

If the toolbar shows "Trace link could not be created", it means the toolbar doesn't have enough information to construct a link. The toolbar needs both an App ID and an Environment name. It tries to auto-detect these from the `/__encore/healthz` endpoint on the request's origin.

1. **Pass parameters explicitly.** The simplest fix is to set `appId` and `envName` directly on the script tag:
   ```html
   <script src="https://encore.dev/encore-toolbar.js?appId=my-app&envName=staging"></script>
   ```

   If you don't want to hardcode the environment name, you can add a backend endpoint that redirects to the toolbar script with the correct parameters:

   ```ts
   import { api } from "encore.dev/api";
   import { appMeta } from "encore.dev";

   export const toolbar = api.raw(
     { method: "GET", expose: true, path: "/encore-toolbar.js" },
     async (req, resp) => {
       const appId = appMeta().appId;
       const envName = appMeta().environment.name;
       const url = `https://encore.dev/encore-toolbar.js?appId=${encodeURIComponent(appId)}&envName=${encodeURIComponent(envName)}`;
       resp.writeHead(302, { Location: url });
       resp.end();
     },
   );
   ```

   Then point your script tag at your own backend instead:
   ```html
   <script src="https://your-api.com/encore-toolbar.js"></script>
   ```

2. **Check that healthz is reachable.** The toolbar auto-detects App ID and environment by calling `/__encore/healthz` on the request's origin. If you have a reverse proxy or API gateway between your frontend and the Encore backend, the `/__encore/healthz` endpoint may not be exposed through it. In that case, either configure your proxy to forward `/__encore/healthz` to the backend, or pass `appId` and `envName` explicitly on the script tag.

3. **Set values in the toolbar.** Open the toolbar's Settings panel and fill in the App ID and Environment fields manually.

### Backend logs are not loading

Backend log streaming only works when running locally. The toolbar connects to the local Encore daemon over WebSocket at `localhost:9400` to fetch logs for a given trace.

1. **Make sure your app is running.** Backend logs require `encore run` to be active.
2. **Check the environment.** Log streaming is only available for the `local` environment. In deployed environments, use the trace link to view logs in [Encore Cloud](https://app.encore.cloud).
3. **Verify the App ID is set.** The toolbar needs a valid App ID to request logs from the daemon. If `/__encore/healthz` is not reachable then set the App ID via the script tag or in the toolbars Settings.
