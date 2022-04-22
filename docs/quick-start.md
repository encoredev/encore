---
title: Quick Start Guide
subtitle: Start building with ease by following this short guide.
---
## Install the Encore CLI
To work with Encore you need our command-line utility, you can install it by running the appropriate command for your system.

**Mac OS:**
```bash
brew install encoredev/tap/encore
```

**Windows:**
```bash
iwr https://encore.dev/install.ps1 | iex
```

**Linux:**
```bash
curl -L https://encore.dev/install.sh | bash
```

## Create your first app
When building with Encore, it’s best to use one application for an entire project.
Create your app by running:
```bash
encore app create
```

Then press `Enter` to create your Encore account, following instructions on screen.

Coming back to the terminal, pick a name for your app.

Then select the `Hello World` app template.

## Run your app locally

Open the folder created for your app, using the app name you picked.
```bash
cd your-app-name
```

Then while in the app root directory, start your app by running:
```bash
encore run
```

You should see this:

```bash
$ encore run
Running on http://localhost:4000
9:00AM INF registered endpoint endpoint=World service=hello
```

While you keep the app running, open a separate terminal and call your API endpoint:

```bash
$ curl http://localhost:4000/hello/world
{"Message": "Hello, world!"}
```

_You've successfully created and run your first Encore application. Well done, you're on your way!_

You can now start using your [local development dashboard](https://encore.dev/docs/observability/dev-dash) by opening [http://localhost:4000](http://localhost:4000) in your browser. Here you can see logs, view traces, and explore the automatically generated API documentation.


## Deploy your app to the cloud

By default your application deploys to Encore's cloud, which is free to use for development and hobby projects.

Deploy your application by running:

```bash
git push encore
```

This single command builds your app, provisions the needed infrastructure, and deploys your application.

_Your app is soon running in the cloud, isn't this exciting?_
  
Now open the [the Encore web application](https://app.encore.dev) to follow your deployment, and once completed you can view production logs and traces, manage environments and connect the cloud account of your choice.

## What's next?

If you're looking for ideas on what to do next, check out the [REST API tutorial](https://encore.dev/docs/tutorials/rest-api).

If you want to chat to other pioneering developers already building with Encore or need help, [join the friendly community on Slack](https://encore.dev/slack).
