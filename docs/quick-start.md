---
title: Quick Start Guide
subtitle: Get started building with Encore in minutes
---

In this short guide, you'll learn Encore's key concepts and get to experience the Encore developer workflow.
It should only take about 5 minutes to complete, and by the end you'll have an API running in the cloud.

<Callout type="important">

To make it easier to follow along, we've laid out a trail of croissants to guide your way.
Whenever you see a 🥐 it means there's something for you to do.

</Callout>

_Let's get started!_

## Install the Encore CLI

To develop locally with Encore, you need the Encore CLI. This is what provisions your local development environment, and runs your local development dashboard complete with logs, tracing, and API documentation.

🥐 Install the Encore CLI by running the appropriate command for your system:

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

When you're building with Encore, it’s best to use one application for an entire project.

🥐 Create your app by running:

```bash
encore app create
```

Since this is the first time you're using Encore, you will be asked to create an account.
After running the above command, press `Enter` to create your Encore account, following the instructions on the screen.
You can use your account with GitHub or Google, or create an account using your email.

Continue by picking a name for your app, using lowercase letters, digits, or dashes.

Then select the `Hello World` template.

This will create an example application, with a simple REST API, in a new folder using the app name you picked.

## Let's take a look at the code

A big part of what makes Encore different is the developer experience when you're writing code.
Let's look at the code to better understand how to build applications with the Encore framework.

🥐 Open the `hello.go` file in your code editor. It's located in the folder: `your-app-name/hello/`.

You should see this:

```go
package hello

import (
	"context"
)

// Welcome to Encore!
// This is a simple "Hello World" project to get you started.
//
// To run it, execute "encore run" in your favorite shell.

// ==================================================================

// This is a simple REST API that responds with a personalized greeting.
// To call it, run in your terminal:
//
//     curl http://localhost:4000/hello/World
//
//encore:api public path=/hello/:name
func World(ctx context.Context, name string) (*Response, error) {
	msg := "Hello, " + name + "!"
	return &Response{Message: msg}, nil
}

type Response struct {
	Message string
}
```

As you can see, it's mostly standard Go code. _(The Encore framework is designed to let you write plain and portable code.)_

One key element is Encore specific, the API annotation:

```bash
//encore:api public path=/hello/:name
```

This annotation is all that's needed for Encore to understand that this Go package is a service, `hello`, and that the `World` function is a public API endpoint.

If you want to create more services and endpoints, it's as easy as creating more Go packages and defining endpoints using the `//encore:api` annotation. _If you're curious, you can read more about [defining services and APIs](/docs/develop/services-and-apis)._

Encore includes a few more native concepts that we'll begin to cover in [the next tutorial](/docs/tutorials/rest-api), which for instance lets you use backend primitives like databases and scheduled tasks by simply writing code.

## Run your app locally

Making it easy to define APIs isn't the only thing Encore does to make your developer experience better.
Encore integrates your entire workflow, from writing code to running in production.

Next, let's try running your application locally.

🥐 Open the folder created for your app, using the app name you picked.

```text
cd your-app-name
```

🥐 Then while in the app root directory, run your app:

```text
encore run
```

You should see this:

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/encorerun.mp4" className="w-full h-full" type="video/mp4" />
</video>

That means your local development environment is up and running!
Encore takes care of setting up all the necessary infrastructure for your applications, including databases.

🥐 While you keep the app running, open a separate terminal and call your API endpoint:

```bash
$ curl http://localhost:4000/hello/world
```

You should see this response:

```text
{"Message": "Hello, world!"}
```

If you see the `Message` response, you've successfully made an API call to your very first Encore application.

_Well done, you're on your way!_

## Your local development dashboard

You can now start using your [local development dashboard](/docs/observability/dev-dash).

🥐 Open [http://localhost:4000](http://localhost:4000) in your browser to access it.

Your development dashboard is a powerful tool to help you move faster when you're developing new features.

It comes with an API explorer with automatically generated documentation, and powerful oberservability features like [distributed tracing](/docs/observability/tracing). _Did we mention Encore automatically instruments your entire application with logs and tracing?_

Through the development dashboard you also have access to [Encore Flow](/docs/develop/encore-flow) which is a visual representation
of your microservice architecture that updates in real-time as you develop your application.

These features may not look like much when viewing such a simple example, but just imagine how much easier it will be to debug your app once it's grown more complex, with multiple services and databases.

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/quickstart_localdevdash.mp4" className="w-full h-full" type="video/mp4" />
</video>

For this example video we have added another service called `foobar` that we call from `hello` to be able to showcase the tracing and architecture visualizations.

## Make a code change

Let's put our mark on this API and make our first code change.

🥐 Head back to your code editor and look at the `hello.go` file again.
If you can't come up with a creative change yourself, why not simply change the "Hello" message to a more sassy "Howdy"?

🥐 Once you've made your change, save the file.

When you save, Encore instantly detects the change and automatically recompiles your application and reloads your local development environment.

The output where you're running your app will look something like this:

```text
Changes detected, recompiling...
Reloaded successfully.
INF registered endpoint endpoint=World path=/hello/:name service=hello
INF listening for incoming HTTP requests
```

🥐 Test your change by calling your API in a separate terminal:

```bash
$ curl http://localhost:4000/hello/world
```

You should now see this response:

```text
{"Message": "Howdy, world!"}
```

## Update your tests

Encore comes with built-in support for [automated testing](/docs/develop/testing) using `encore test`, built on top of Go's `go test` functionality.
Encore will automatically run tests when you trigger a deployment, so before you can deploy your changes to the cloud, you first need to update the tests.

🥐 Try running `encore test ./...` to run tests in all sub-directories, or just `encore test` for the current directory.

You should now see you have a failing test. Let's fix that.

🥐 Open the `hello_test.go` file which has the tests for the `hello` service.

You should see this:

```go
package hello

import (
	"context"
	"testing"
)

func TestWorld(t *testing.T) {
	resp, err := World(context.Background(), "Jane Doe")
	if err != nil {
		t.Fatal(err)
	}
	want := "Hello, Jane Doe!"
	if got := resp.Message; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
```

🥐 Next, update the test to match your changes, by updating `TestWorld`. In our example:

```go
    want := "Howdy, Jane Doe!"
```

🥐 Save the file and run `encore test ./...` to verify it's now working.

Great job, you're now ready to head to the cloud!

## Deploy your app to the cloud

_Remember we said Encore integrates your entire workflow? Let's try it out!_

The first time you deploy, Encore will by default create a staging [environment](/docs/deploy/environments) in Encore's cloud, running on Google Cloud Platform. This is free to use for development and hobby projects.

When you're ready, you can [deploy to your own cloud](/docs/deploy/own-cloud) using your own cloud account with GCP/AWS/Azure. Or even all of them – Encore makes it seamless to deploy to multiple cloud environments all at once.

🥐 Now let's head to the cloud! Push your changes and deploy your application by running:

```bash
$ git add -A .
$ git commit -m 'Initial commit'
$ git push encore
```

Encore will now build and test your app, provision the needed infrastructure, and deploy your application to the cloud.

_Your app will soon be running in the cloud, isn't this exciting?_

## Head to the web platform

After triggering the deployment, you will see a url where you can view its progress in the Encore web platform.
It will look something like: `https://app.encore.dev/$APP_ID/deploys/staging/$DEPLOY_ID`

🥐 Open the url to access the web platform and check the progress of your deployment.

You can now use the web platform to view production [logs](/docs/observability/logging) and [traces](/docs/observability/tracing), create new [environments](/docs/deploy/environments), connect the [cloud account of your choice](/docs/deploy/own-cloud), [integrate with GitHub](/docs/how-to/github), and much more.

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/deployment.mp4" className="w-full h-full" type="video/mp4" />
</video>

## Call your API in the cloud

Now that you've created your staging environment, you're ready to call your API running in the cloud.
Your API Base URL will be something like: `https://staging-$APP_ID.encr.app`

🥐 When the deployment is finished, call your API from the terminal (replacing `$APP_ID` with your own App ID):

```text
$ curl https://staging-$APP_ID.encr.app/hello/world
```

You should get this response:

```text
{"Message": "Howdy, world!"}
```

If you see the `Message` response, you've successfully made an API call to your very first Encore app running in the cloud.

_Congratulations, you're well on your way to escaping the maze of cloud complexity!_

## What's next?

🥐 Check out the [REST API tutorial](/docs/tutorials/rest-api) to learn how to add more services, use databases, and write tests, all in just a few minutes.

If you want to chat to other pioneering developers already building with Encore, or need help, join the friendly community on [Slack](/slack).
