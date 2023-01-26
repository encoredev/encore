---
seotitle: How to create your first backend application with Encore
seodesc: Learn how to create a Go based cloud backend application using Encore. Get a backend running in the cloud i 5 minutes!
title: Create a new Encore app
---

In this tutorial you will create your very first Encore application,
and deploy it to production using the Encore platform.

## 1. Make sure Encore is installed
If you haven't already done so, you'll need to install Encore.
Install it by running the appropriate command for your system.

Mac OS:
```shell
$ brew install encoredev/tap/encore
```

Windows:
```shell
$ iwr https://encore.dev/install.ps1 | iex
```

Linux:
```shell
$ curl -L https://encore.dev/install.sh | bash
```

You can check that everything's working by running `encore version` in your terminal.
It should print something like:
```shell
$ encore version v1.0.0
```


## 2. Create a new app
Once you've installed Encore, creating an app is easy, simply run:
```shell
$ encore app create
```

Then pick a name for your app using lowercase letters, digits, or dashes.

Finally, select the `Hello World` template.

## 3. Running your app

To run your app, open the folder created for your application using the name of your app.
```shell
$ cd your-app-name
```

Then while in the app root directory, run your app by simply running:
```shell
$ encore run
```

You should see this:

```shell
$ encore run
Running on http://localhost:4000
9:00AM INF registered endpoint endpoint=World service=hello
```

That means your application is up and running!

While you keep the app running, open a separate terminal and call your API endpoint:

```shell
$ curl http://localhost:4000/hello/world
{"Message": "Hello, world!"}
```

If you see this message, you've successfully created and run your first Encore application.
Well done! Let's deploy it to the cloud.

## 4. Deploy your app to the cloud

Deploying your app to the cloud is as easy as running:

```shell
$ git push encore
```
This will trigger a build and deploy. You'll see the deploy logs being streamed directly to your terminal.

Once the deploy completes, your app is up and running in production!

Take note of your API Base URL that will be something like: `https://$APP_ID.encoreapi.com/prod`

## 5. Call your API
To verify that it's running, let's call our API.

Now, open your terminal and run (replace `$APP_ID` with your own App ID):

```shell
$ curl https://$APP_ID.encoreapi.com/prod/hello/world
{"Message": "Hello, world!"}
```

If you see this, you've successfully deployed and made an API call to your very first Encore app in production.
Nicely done!

There's lots more to see in Encore's cloud platform. Head over to [app.encore.dev](https://app.encore.dev)
and open your app to explore further!
