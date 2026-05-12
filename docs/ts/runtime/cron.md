---
title: encore.dev/cron
lang: ts
toc: true
---

# encore.dev/cron

## Classes

### CronJob

Defined in: [cron/mod.ts:5](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L5)

#### Constructors

##### Constructor

```ts
new CronJob(name, cfg): CronJob;
```

Defined in: [cron/mod.ts:8](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L8)

###### Parameters

###### name

`string`

###### cfg

[`CronJobConfig`](#cronjobconfig)

###### Returns

[`CronJob`](#cronjob)

#### Properties

##### cfg

```ts
readonly cfg: CronJobConfig;
```

Defined in: [cron/mod.ts:7](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L7)

##### name

```ts
readonly name: string;
```

Defined in: [cron/mod.ts:6](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L6)

## Type Aliases

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

Defined in: [cron/mod.ts:14](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/cron/mod.ts#L14)

#### Type Declaration

##### endpoint

```ts
endpoint: () => Promise<unknown>;
```

###### Returns

`Promise`\<`unknown`\>

##### title?

```ts
optional title?: string;
```
