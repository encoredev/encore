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

When you need to run periodic and recurring tasks, Encore.ts provides a declarative way of using Cron Jobs.

When a Cron Job is defined in your application, Encore automatically calls your specified API according to the defined schedule. This eliminates the need for infrastructure maintenance, as Encore manages scheduling, monitoring, and execution of Cron Jobs.

<Callout type="info">

Cron Jobs do not run when developing locally or in [Preview Environments](/docs/platform/deploy/preview-environments), but you can always call the API manually to test the behavior.

</Callout>

<GitHubLink
    href="https://github.com/encoredev/examples/tree/main/ts/uptime"
    desc="Uptime Monitoring app that uses a Cron Job to periodically check the uptime of a website."
/>

## Defining a Cron Job

To define a Cron Job, import `encore.dev/cron` and call `new CronJob`, assigning the result to a top-level variable.

### Example

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

The Encore Cloud dashboard provides a convenient user interface for monitoring and debugging
Cron Job executions across all your environments via the `Cron Jobs` menu item:

![Cron Jobs UI](/assets/docs/cron.png)

## Keep in mind when using Cron Jobs

- Cron Jobs do not execute during local development or in [Preview Environments](/docs/platform/deploy/preview-environments). However, you can manually invoke the API to test its behavior.
- In Encore Cloud, Cron Job executions are limited to **once every hour**, with the exact minute randomized within that hour for users on the Free Tier. To enable more frequent executions or to specify the exact minute within the hour, consider [deploying to your own cloud](/docs/platform/deploy/own-cloud) or upgrading to the [Pro plan](/pricing).
- Both public and private APIs are supported for Cron Jobs.
- Ensure that the API endpoints used in Cron Jobs are idempotent, as they may be called multiple times under certain network conditions.
- API endpoints utilized in Cron Jobs must not accept any request parameters.

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
