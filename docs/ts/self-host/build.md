---
seotitle: Build Docker Images
seodesc: Learn how to build Docker images for your Encore application, which can be self-hosted on your own infrastructure.
title: Build Docker Images
lang: ts
---

Encore supports building Docker images directly from the CLI, which can then be self-hosted on your own infrastructure of choice.

This can be a good choice if [Encore Cloud](/docs/platform) isn't a good fit for your use case, or if you want to [migrate away](/docs/ts/migration/migrate-away).

## Building your own Docker image

To build your own Docker image, use `encore build docker MY-IMAGE:TAG` from the CLI.

This will compile your application using the host machine and then produce a Docker image containing the compiled application. The base image defaults to `scratch` for GO apps and `node:slim` for TS, but can be customized with `--base`.

This is exactly the same code path that Encore's CI system uses to build Docker images, ensuring compatibility.

By default, all your services are included and started by the Docker image. If you want to specify specific services and gateways to include, you can use the `--services` and `--gateways` flags.

```bash
encore build docker --services=service1,service2 --gateways=api-gateway MY-IMAGE:TAG
```

The image will default to run on port 8080, but you can customize it by setting the `PORT` environment variable when starting your image.

```bash
docker run -e PORT=8081 -p 8081:8081 MY-IMAGE:TAG
```

Congratulations, you've built your own Docker image! 🎉
Continue to learn how to [configure infrastructure](/docs/ts/self-host/configure-infra).
