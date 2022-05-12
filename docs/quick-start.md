---
title: Quick Start Guide
subtitle: Start building with ease by following this short guide.
---
## Install the Encore CLI
To develop locally with Encore, you need the Encore CLI. This is what provisions your local development environment, and runs your local development dashboard complete with logs, tracing, and API documentation.

Install the Encore CLI by running the appropriate command for your system.

**Mac OS**
```bash
brew install encoredev/tap/encore
```

**Windows**
```bash
iwr https://encore.dev/install.ps1 | iex
```

**Linux**
```bash
curl -L https://encore.dev/install.sh | bash
```

You can check that everything's working by running `encore version` in your terminal.
It should print something like:
```bash
encore version v1.0.0
```

## Create your first app
When building with Encore, it’s best to use one application for an entire project. Create your app by running:
```bash
encore app create
```

Because this is the first time you're using Encore, you will be asked to create an account.
After running the above command, press `Enter` to create your Encore account, following the instructions on screen.

Coming back to the terminal, continue by picking a name for your app, using lowercase letters, digits, or dashes.

Then select the `Hello World` template.

## Run your app locally

Open the folder created for your app, using the app name you picked.
```bash
cd your-app-name
```

Then while in the app root directory, run your app by simply running:
```bash
encore run
```

You should see this:

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/encorerun.mp4" className="w-full h-full" type="video/mp4" />
</video>

That means your application is up and running!

While you keep the app running, open a separate terminal and call your API endpoint:

```bash
$ curl http://localhost:4000/hello/world
{"Message": "Hello, world!"}
```

_You've successfully created and run your first Encore application. Well done, you're on your way!_

You can now start using your [local development dashboard](/docs/observability/dev-dash) by opening [http://localhost:4000](http://localhost:4000) in your browser. Here you can see logs, view traces, and explore the automatically generated API documentation.

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/quickstart_localdevdash.mp4" className="w-full h-full" type="video/mp4" />
</video>

## Deploy your app to the cloud

By default Encore creates a staging [environment](/docs/deploy/environments) in Encore's cloud, which is free to use for
development and hobby projects. You can also [deploy to your own cloud](/docs/deploy/own-cloud), and create different types of environments using different clouds.

Let's head to the cloud! Deploy your application by running:

```bash
git push encore
```

This single command builds your app, provisions the needed infrastructure, and deploys your application to the cloud
directly from your terminal.

After triggering the deployment, you will see a url where you can view its progress in the Encore web platform.
It will look something like:  `https://app.encore.dev/$APP_ID/deploys/staging/$DEPLOY_ID`

Open the url to access the web platform and check the progress of your deployment.

Once the deployment is completed, you can use the [web platform](https://app.encore.dev) to view production [logs](/docs/observability/logging) and [traces](/docs/observability/tracing), create new [environments](/docs/deploy/environments) and connect the [cloud account of your choice](/docs/deploy/own-cloud).

*Your app will soon be running in the cloud, isn't this exciting?*

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/deployment.mp4" className="w-full h-full" type="video/mp4" />
</video>

## Call your API
Your API Base URL will be something like: `https://staging-$APP_ID.encr.app/hello/world`.

Once the deployment is completed, you can call your API by opening your terminal and running (replacing `$APP_ID` with your own App ID):

```bash
$ curl https://staging-$APP_ID.encr.app/hello/world
{"Message": "Hello, world!"}
```

If you see this, you've successfully made an API call to your very first Encore app running in the cloud.

*Congratulations, you're well on your way to escaping the maze of cloud complexity!*

## What's next?

Check out the [REST API tutorial](/docs/tutorials/rest-api) to learn how to create a cloud backend, complete with a database and tests, in just a few minutes.

If you want to chat to other pioneering developers already building with Encore, or need help, join the friendly community on [Slack](/slack).
