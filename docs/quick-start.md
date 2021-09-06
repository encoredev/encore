---
title: Quick Start Guide
subtitle: Get started with Encore in minutes
---
## Install the Encore CLI
To work with Encore you need our command-line utility.

Install by running the appropriate command for your system.

Mac OS:
```bash
brew install encoredev/tap/encore
```

Windows:
```bash
iwr https://encore.dev/install.ps1 | iex
```

Linux:
```bash
curl -L https://encore.dev/install.sh | bash
```

## Create your app
Create your app by running:
```bash
encore app create
```

Then press `Enter` to create your free account, following instructions on screen.

Coming back to the terminal, pick a name for your app.

Then select the `Hello World` app template.

## Run your app locally

Open the folder created for your app, using the app name you picked.
```bash
cd your-app-name
```

Then while in the app root directory, run your app.
```bash
encore run
```

You should see this:

```bash
$ encore run
Running on http://localhost:4060
9:00AM INF registered endpoint endpoint=World service=hello
```

While you keep the app running, open a separate terminal and call your API endpoint:

```bash
$ curl http://localhost:4060/hello.World
{"Message": "Hello, world!"}
```

_You've successfully created and run your first Encore application. Well done! 🎉_

You can now access your local development dashboard by opening [http://localhost:4060](http://localhost:4060) in your browser. Here you can monitor logs, view traces, and explore the automatically generated API documentation.


## Deploy your app

By default your application deploys to Encore's cloud, which is free to use for development and hobby projects.

Deploy your application by running:

```bash
git push encore
```

_You now have an app running in the cloud. Congratulations! 🚀_
  
Now open the [the Encore web application](https://app.encore.dev) where you can see production logs and traces, manage environments and configure the cloud hosting of your choice.

## What's next?

If you're looking for ideas on what to do next, check out the [REST API tutorial](https://encore.dev/docs/tutorials/rest-api).

If you want to chat to others building with Encore or need help, [join our friendly community on Slack](https://encore.dev/slack).
