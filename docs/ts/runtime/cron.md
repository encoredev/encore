---
title: encore.dev/cron
lang: ts
toc: true
---

## Classes

<!-- symbol-start: CronJob -->
### CronJob

<!-- source: cron/mod.ts:5 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L5)

#### Constructors

##### Constructor

`new CronJob(name, cfg): CronJob`

<!-- source: cron/mod.ts:8 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L8)

###### Parameters

###### name

`string`

###### cfg

[`CronJobConfig`](#cronjobconfig)

###### Returns

[`CronJob`](#cronjob)

#### Properties

##### cfg

`readonly cfg: CronJobConfig`

<!-- source: cron/mod.ts:7 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L7)

##### name

`readonly name: string`

<!-- source: cron/mod.ts:6 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L6)

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: CronJobConfig -->
### CronJobConfig

```ts
type CronJobConfig = {
  endpoint: () => Promise<unknown>;
  title?: string;
} & 
  | {
  every: DurationString;
}
  | {
  schedule: string;
};
```

<!-- source: cron/mod.ts:14 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L14)

#### Type Declaration

##### endpoint

`endpoint: () => Promise<unknown>`

###### Returns

`Promise`\<`unknown`\>

##### title?

`optional title?: string`


<!-- symbol-end -->