---
title: Cron Jobs
---

When building backend applications it's a common need to run periodic and recurring tasks.
For example, to send a welcome email to everyone who signed up recently.
Encore provides native support for these types of use cases using **Cron Jobs**.

When a cron job is defined, the Encore Platform will call the API of your choice on the schedule you have defined.
There is no need to maintain any infrastructure; Encore itself takes care of the scheduling, monitoring and execution.

## Defining a Cron job

To define a cron job, all you need to do is to import the `encore.dev/cron` package,
and call the `cron.NewJob()` function and store it as a package-level variable:

```go
import "encore.dev/cron"

// Send a welcome email to everyone who signed up in the last two hours.
var _ = cron.NewJob("welcome-email", cron.JobConfig{
	Name:     "Send welcome emails",
	Every:    2 * cron.Hour,
	Endpoint: SendWelcomeEmail,
})

// SendWelcomeEmail emails everyone who signed up recently.
// It's idempotent: it only sends a welcome email to each person once.
//encore:api private
func SendWelcomeEmail(ctx context.Context) error {
	// ...
	return nil
}
```

The `"welcome-email"` argument to `cron.NewJob` is a unique ID you give to each cron job.
If you later refactor the code and move the cron job definition to another package,
we use this ID to keep track that it's the same cron job and not a different one.

When this code gets deployed the Encore Platform will automatically register the cron job
and begin calling the `SendWelcomeEmail` API every hour.

The Encore Platform provides a convenient user interface for monitoring and debugging
cron job executions across all your environments via the `Cron Jobs` menu item:

![Cron Jobs UI](/assets/docs/cron.png)

<Callout type="important">

A few important things to know:

- Cron jobs work across all the cloud providers Encore supports, and support both public and private APIs.
- Cron jobs do not run when developing locally; you can always call the API manually to test it.
- The API endpoints used in cron jobs should always be idempotent. It's possible they're called multiple times in some network conditions.
- The API endpoints used in cron jobs must not take any request parameters. That is, their signatures must be `func(context.Context) error` or `func(context.Context) (*T, error)`.

</Callout>

## Cron schedules

Above we used the `Every` field, which executes the cron job on a periodic basis.
It runs around the clock each day, starting at midnight (UTC).

In order to ensure a consistent delay between each run, the interval used **must divide 24 hours evenly**.
For example, `10 * cron.Minute` and `6 * cron.Hour` are both allowed (since 24 hours is evenly divisible by both),
whereas `7 * cron.Hour` is not (since 24 is not evenly divisible by 7).
The Encore compiler will catch this and give you a helpful error at compile-time if you try to use an invalid interval.

### Cron expressions

For more advanced use cases, such as running a cron job on a specific day of the month, or a specific week day, or similar,
the `Every` field is not expressive enough.

For these use cases, Encore provides full support for [cron expressions](https://en.wikipedia.org/wiki/Cron) by using the `Schedule` field
instead of the `Every` field.

For example:

```go
// Run the monthly accounting sync job at 4am (UTC) on the 15th day of each month.
var _ = cron.NewJob("accounting-sync", cron.JobConfig{
	Name:     "Cron Job Example",
	Schedule: "0 4 15 * *",
	Endpoint: AccountingSync,
})
```
