---
seotitle: Build Docker Images
seodesc: Learn how to build Docker images for your Encore application, which can be self-hosted on your own infrastructure.
title: Build Docker Images
lang: ts
---

Encore supports building Docker images directly from the CLI, which can then be self-hosted on your own infrastructure of choice.

This can be a good choice if [Encore Cloud](/docs/platform) isn't a good fit for your use case, or if you want to [migrate away](/docs/ts/migration/migrate-away).

## Feature parity

Some aspects of how Encore Cloud and Encore Local Dashboard work are not applicable for self-hosted instances - for example, when you self-host, you should provide your own database container, and do SQL migrations manually. But do not worry, it would be explained further how to do it!

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

## How to do SQL migrations

 For PostgreSQL, it is easily done with the help of separate one-shot container, based on postgres image you would already have for your DB. This is a minimal example with all services configured (except for PostgreSQL) correctly to ensure successfull migration workflow:

**docker_compose.yml:**
```yaml
services:
  migrate:
    image: postgres:16-alpine
    container_name: your-api-migrate
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      PGPASSWORD: YOUR_PASSWORD
    volumes:
      - ./your-api/src/services/migrations:/migrations:ro
      - ./scripts/migrate.sh:/migrate.sh:ro
    command: ["sh", "/migrate.sh"]
  your-api:
    image: MY-IMAGE:TAG
    container_name: your-api-encore
    depends_on:
      postgres:
        condition: service_healthy
      migrate:
        condition: service_completed_successfully
  postgres:
    image: postgres:16-alpine
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5
    #...other postgres-related options
```

And `migrate.sh` would be the script, used for actual migration.
Minimal viable example:

```bash
for f in /migrations/*.up.sql; do
  [ -f "$f" ] || continue
  log "[migrate] applying [$f]"
  # -v ON_ERROR_STOP=1 makes psql exit non-zero on first error.
  psql -v ON_ERROR_STOP=1 -h postgres -U postgres -d your-api_db -f "$f"
done
```

Insides of the script can be replaced depending on what migration tool you want to use.

Congratulations, you've built your own Docker image! 🎉
Continue to learn how to [configure infrastructure](/docs/ts/self-host/configure-infra).
