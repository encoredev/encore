---
title: encore.dev/cron
lang: ts
toc: true
---

## Classes

<!-- symbol-start: CronJob -->
### CronJob [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L5)

#### Constructors

##### Constructor [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L8)

`new CronJob(name, cfg): CronJob`

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

##### name

`readonly name: string`

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: CronJobConfig -->
### CronJobConfig [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L14)

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

#### Type Declaration

##### endpoint

`endpoint: () => Promise<unknown>`

###### Returns

`Promise`\<`unknown`\>

##### title?

`optional title?: string`


<!-- symbol-end -->