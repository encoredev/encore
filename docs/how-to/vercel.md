---
seotitle: Use Encore together with Vercel for frontend hosting
seodesc: Learn how to use an Encore backend together with a frontend hosted by Vercel.
title: Use Vercel for frontend hosting
---

Encore is not opinionated about where you host your frontend, pick the platform that suits your situation best. In this
guide, we'll show you how to use Vercel to host your frontend and Encore to host your backend.

Take a look at our **Encore + Next.js starter** for an example: https://github.com/encoredev/nextjs-starter

## Folder structure
If you want to go for a monorepo approach, you can place your frontend and backend in separate top-level folders, like so:

```
/my-app
├── backend
│   ├── encore.app
│   ├── package.json // Backend dependencies
│   └── ...
└── frontend
    ├── package.json // Frontend dependencies
    └── ...
```

This way, you can keep your frontend and backend dependencies separate, while still having the codebases in the same repository.

## Deployment

Both Encore and Vercel support automatic deploys from GitHub. This means that you can push your code to GitHub and have
both your frontend and backend automatically deployed.

### Encore

1. Open your app in the Encore [Cloud Dashboard](https://app.encore.dev).
2. Go to your app settings and set the "Root Directory" to `backend`. We need to do this because the `encore.app` file is not in the repo root (given the folder structure suggested above).
3. In the app settings as well, link your app to GitHub and select the repo you just created.

Whenever you push a change to GitHub you will trigger a deploy.

#### Preview Environments for each Pull Request

Once you've linked your app with GitHub, Encore will automatically start building and running tests against
your Pull Requests.

Encore will also provision a dedicated Preview Environment for each pull request.
This environment works just like a regular development environment, and lets you test your changes
before merging.

Learn more in the [Preview Environments documentation](/docs/deploy/preview-environments).

![Preview environment linked in GitHub](/assets/docs/ghpreviewenv.png "Preview environment linked in GitHub")

### Vercel

1. Create a new project on Vercel and point it to your GitHup repo.
2. Select `frontend` as the root directory for the Vercel project (given the folder structure suggested above).

## CORS configuration

If you are running into CORS issues when calling your Encore API from your frontend then you may need to specify which
origins are allowed to access your API (via browsers). You do this by specifying the `global_cors` key in the `encore.app`
file, which has the following structure:

```js
global_cors: {
  // allow_origins_without_credentials specifies the allowed origins for requests
  // that don't include credentials. If nil it defaults to allowing all domains
  // (equivalent to ["*"]).
  "allow_origins_without_credentials": [
    "<ORIGIN-GOES-HERE>"
  ],
        
  // allow_origins_with_credentials specifies the allowed origins for requests
  // that include credentials. If a request is made from an Origin in this list
  // Encore responds with Access-Control-Allow-Origin: <Origin>.
  //
  // The URLs in this list may include wildcards (e.g. "https://*.example.com"
  // or "https://*-myapp.example.com").
  "allow_origins_with_credentials": [
    "<DOMAIN-GOES-HERE>"
  ]
}
```

Learn more in the [CORS documentation](/docs/develop/cors).
