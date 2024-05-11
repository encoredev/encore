---
seotitle: Create recurring tasks with Encore's Cron Jobs API
seodesc: Learn how to create periodic and recurring tasks in your backend application using Encore's Cron Jobs API.
title: Cron Jobs
subtitle: Run recurring and scheduled tasks
infobox: {
  title: "Cron Jobs",
  import: "encore.dev/cron",
  example_link: "/docs/tutorials/uptime"
}
lang: ts
---

When you need to run periodic and recurring tasks, Encore's Backend SDK provides a declarative way of using Cron Jobs.

When a Cron Job is defined, Encore will call the API of your choice on the schedule you have defined.
This means there is no need to maintain any infrastructure, as Encore handles the scheduling, monitoring and execution of Cron Jobs.

## Defining a Cron Job

To define a Cron Job, import `encore.dev/cron` and call `new CronJob`, assigning the result to a top-level variable.

For example:

```ts
import { CronJob } from "encore.dev/cron";
import { api } from "encore.dev/api";

// Send a welcome email to everyone who signed up in the last two hours.
const _ = new CronJob("welcome-email", {
	title: "Send welcome emails",
	every: "2h",
	endpoint: sendWelcomeEmail,
})

// Emails everyone who signed up recently.
// It's idempotent: it only sends a welcome email to each person once.
export const sendWelcomeEmail = api({}, async () => {
	// Send welcome emails...
});
```

The `"welcome-email"` argument to `new CronJob` is a unique ID you give to each Cron Job.
If you later refactor the code and move the Cron Job definition to another package,
Encore uses this ID to keep track that it's the same Cron Job and not a different one.

When this code gets deployed Encore will automatically register the Cron Job in Encore Cloud
and begin calling the `sendWelcomeEmail` API every two hours.

Encore's Cloud Dashboard provides a convenient user interface for monitoring and debugging
Cron Job executions across all your environments via the `Cron Jobs` menu item:

![Cron Jobs UI](/assets/docs/cron.png)

A few important things to know:

- Cron Jobs do not run when developing locally or in [Preview Environments](/docs/deploy/preview-environments); but you can always call the API manually to test the behavior.
- Cron Jobs execution in Encore Cloud is capped at **once every hour** and the minute is randomized within the hour that they run for users on the Free Tier; [deploy to your own cloud](/docs/deploy/own-cloud) or upgrade to the [Pro plan](/pricing) to use more frequent executions or to set the minute within the hour when the job runs.
- Cron Jobs support both public and private APIs.
- The API endpoints used in Cron Jobs should always be idempotent. It's possible they're called multiple times in some network conditions.
- The API endpoints used in Cron Jobs must not take any request parameters.

## Cron schedules

Above we used the `every` field, which executes the Cron Job on a periodic basis.
It runs around the clock each day, starting at midnight (UTC).

In order to ensure a consistent delay between each run, the interval used **must divide 24 hours evenly**.
For example, `10m` and `6h` are both allowed (since 24 hours is evenly divisible by both),
whereas `7h` is not (since 24 is not evenly divisible by 7).
The Encore compiler will catch this and give you a helpful error at compile-time if you try to use an invalid interval.

### Cron expressions

For more advanced use cases, such as running a Cron Job on a specific day of the month, or a specific week day, or similar,
the `every` field is not expressive enough.

For these use cases, Encore provides full support for [Cron expressions](https://en.wikipedia.org/wiki/Cron) by using the `schedule` field
instead of the `every` field.

For example:

```ts
// Run the monthly accounting sync job at 4am (UTC) on the 15th day of each month.
const _ = new CronJob("accounting-sync", {
	title:    "Cron Job Example",
	schedule: "0 4 15 * *",
	endpoint: accountingSync,
})
```
