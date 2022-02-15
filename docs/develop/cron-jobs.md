---
title: Cron Jobs
---

You can use a CronJob to run jobs on a time-based schedule. These automated jobs run similarly to [Cron](https://en.wikipedia.org/wiki/Cron) tasks.

Cron jobs are useful for creating periodic and recurring tasks, like running a database query or sending emails. Cron jobs can also schedule individual tasks for a specific time, such as if you want to schedule a job for a low activity period.

<Callout type="important">

**Cron jobs** have limitations and idiosyncrasies. For example, in certain circumstances, a single cron job can create multiple jobs. Therefore, jobs should be **idempotent**.

</Callout>

## Defining a Cron job

With Encore you define cron jobs directly in your code, using our infrastructure declaration syntax. We believe it’s forward-compatible and has the easiest path to discoverability.

```go
package hello // service name

import (
	"context"
	"encore.dev/cron"
)

/*
  In this example, we are defining a cron job.
  The comment above the cron job definition will 
  be used as a description.

  You can also add code snippets in your description,
  which will be picked up by our parser and displayed
  in our web UI.

    var cfg := cron.JobConfig{
      Name:     "Cron Job Example",
      Every:    5 * time.Minute,
      Endpoint: Cron
    }
*/
var _ = cron.NewJob("cronjobexample", cron.JobConfig{
	Name:     "Cron Job Example",
	Every:    5 * time.Minute,
	Endpoint: Cron,
})

//encore:api public path=/cron
func Cron(ctx context.Context) (*Response, error) {
	msg := "Hello, Cron!"
	return &Response{Message: msg}, nil
}

type Response struct {
	Message string
}
```

<Callout type="important">

We only support using infrastructure creation at the package level, any other call location will result in a compilation error.

</Callout>

## Cron job configuration

- `cron.NewJob(ID, ...)` - This ID allows you to rename the variable _or_ move the infrastructure resource without having it being destroyed/recreated. It would cause a compilation error to have two resources of the same type with the same ID within the app.
- `Name` - The Cron job user friendly name, will auto complete based on the **ID** if not set.
- `Every` - `time.Duration` that determines how often the Cron job is executed; <sup>*</sup>*if you want more control over the execution schedule we support traditional [cron expressions](https://en.wikipedia.org/wiki/Cron#CRON_expression) by setting the `Schedule` field in the `JobConfig` struct; we support either **Every** or **Schedule** but not both*.
- `Endpoint` - The Encore API endpoint containing the actual code you want to run periodically.

<Callout type="important">

We only support endpoints that dont't have any request data. As previously stated, these should be idempotent. 

</Callout>

That means there are only two ways of defining an API endpoint for a Cron job:

- `func Foo(ctx context.Context) (*Response, error)` – when you only return a response
- `func Foo(ctx context.Context) error` – when you need neither request nor response data

## Monitoring Cron jobs

After deploying your app, all Cron jobs defined in code will be shown in your Encore dashboard.

You'll be able to filter them by environment, as well as see if they're active or not, what's the status of their last execution and when are they scheduled to run next.

![Cron Jobs UI](/assets/docs/cron.png)
