---
seotitle: Use Encore together with NestJS
seodesc: Learn how to use NestJS to structure your business logic and Encore for creating infrastructure resources.
title: Use NestJS with Encore
lang: ts
---

[Nest](https://docs.nestjs.com/) (NestJS) is a framework for building efficient, scalable TypeScript server-side
applications. Nest aims to provide
an application architecture out of the box which allows for effortless creation of highly testable, scalable, and
loosely coupled and easily maintainable applications.

Encore is not opinionated when it comes to application architecture, so you can use it together with NestJS to structure
your business logic and Encore for creating backend primitives like APIs, Databases, and Cron Jobs.

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/ts/nestjs" 
    desc="Encore.ts + NestJS example" 
/>

## Adding Encore to a NestJS project

If you already have a NestJS project, you can add Encore to it by following these steps:

1. Run `encore app init` in the root of your project to create a new Encore application.
2. Add `encore.dev` as a dependency by running `npm install encore.dev`.
3. Add the following `paths` to your `tsconfig.json`:

```json
-- tsconfig.json --
{
   "compilerOptions": {
      "paths": {
         "~encore/*": [
            "./encore.gen/*"
         ]
      }
   }
}
```

## Standalone Nest application

In order for Encore to be able to provision infrastructure resources, generate API documentation etc. we need to run our
application using Encore. This means that we need to replace the ordinary Nest bootstrapping and instead run our Nest
app as
a [standalone application](https://docs.nestjs.com/standalone-applications). We do this by
calling `NestFactory.createApplicationContext(AppModule)` and then selecting the modules/services we need:

```ts
-- applicationContext.ts --
const applicationContext: Promise<{ catsService: CatsService }> =
  NestFactory.createApplicationContext(AppModule).then((app) => {
    return {
      catsService: app.select(CatsModule).get(CatsService, {strict: true}),
      // other services...
    };
  });

export default applicationContext;
```

The `applicationContext` variable can then be used to access your Nest modules/services from your Encore your APIs.

## Defining an Encore service

When running an app using Encore you need at least
one [Encore service](/docs/ts/primitives/services#defining-a-service). You can define a
service
in two ways:

1. Create a folder and inside that folder defining one or more APIs. Encore recognizes this as a service, and uses the
   folder name as the service name.
2. Add a file named `encore.service.ts` in a directory. The file must export a service instance, by
   calling `new Service`, imported from `encore.dev/service`:

```ts
import {Service} from "encore.dev/service";

export default new Service("my-service");
```

Encore will consider this directory and all its subdirectories as part of the service.

If you already have a Nest app then the easiest way to get going is to go with the second approach and add
a `encore.service.ts` in the root of your app, then you do not need to change your existing folder structure. 

## Replacing Nest controllers with Encore APIs

If you already have a Nest app then you can keep most of your business logic (modules, services and providers) as is but
in order for Encore to be able to manage your APIs, you need to replace your Nest controllers with Encore APIs.

Let's assume you have a `cats/cats.controller.ts` in your Nest app that looks like this:

```ts
-- cats/cats.controller.ts --

@Controller('cats')
export class CatsController {
  constructor(private readonly catsService: CatsService) {
  }

  @Post()
  @Roles(['admin'])
  async create(@Body() createCatDto: CreateCatDto) {
    this.catsService.create(createCatDto);
  }

  @Get()
  async findAll(): Promise<Cat[]> {
    return this.catsService.findAll();
  }

  @Get(':id')
  findOne(
    @Param('id', new ParseIntPipe())
      id: number,
  ) {
    return this.catsService.get(id);
  }
}
```

When converting this to using Encore it would look like this:

```ts
-- cats/cats.controller.ts --
export const findAll = api(
  {expose: true, method: 'GET', path: '/cats'},
  async (): Promise<{ cats: Cat[] }> => {
    const {catsService} = await applicationContext;
    return {cats: await catsService.findAll()};
  },
);

export const get = api(
  {expose: true, method: 'GET', path: '/cats/:id'},
  async ({id}: { id: number }): Promise<{ cat: Cat }> => {
    const {catsService} = await applicationContext;
    return {cat: await catsService.get(id)};
  },
);

export const create = api(
  {expose: true, auth: true, method: 'POST', path: '/cats'},
  async (dto: CreateCatDto): Promise<void> => {
    const {catsService} = await applicationContext;
    catsService.create(dto);
  },
);
```

We use the `applicationContext` (that we defined above) to access our `catsService` and pass in the necessary
parameters.

Both Encore and Nest use the concept of a `service`. With Encore you define a service by creating a folder and inside
that folder defining one or more APIs. Encore recognizes this as a service, and uses the folder name as the service
name. When deploying, Encore will automatically provision the required infrastructure for each service. So in the
example
above we have a `cats` service with three APIs because `cats.controller.ts` is placed inside a folder named `cats`.

## Making use of other Encore features

Encore also allows you to easily make use of other backend primitives in your Nest app,
like [Databases](/docs/ts/primitives/databases), [Cron Jobs](/docs/ts/primitives/cron-jobs), [Pub/Sub & Queues](/docs/ts/primitives/pubsub)
and [Secrets](/docs/ts/primitives/secrets).

Take a look at our [Encore + NestJS example](https://github.com/encoredev/examples/tree/main/ts/nestjs) which uses both
a PostgreSQL Database and an [Auth Handler](/docs/ts/develop/auth) to authenticate incoming requests.

## Running your Encore app

After those steps we are ready to run our app locally:

```shell
$ encore run
```

You should see log messages about both Encore and Nest staring up. That means your local development environment is up
and
running and ready to take some requests!

### Open the Local Development Dashboard

You can now start using your [Local Development Dashboard](/docs/ts/observability/dev-dash).

Open [http://localhost:9400](http://localhost:9400) in your browser to access it.

<video autoPlay playsInline loop controls muted className="w-full h-full">
  <source src="/assets/docs/localdashvideo.mp4" className="w-full h-full" type="video/mp4"/>
</video>

The Local Development Dashboard is a powerful tool to help you move faster when you're developing new features.

It comes with an API explorer, a Service Catalog with automatically generated documentation, and powerful
observability features
like [distributed tracing](/docs/ts/observability/tracing).
