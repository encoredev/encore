---
seotitle: Using Encore with Turborepo in a monorepo
seodesc: Learn how to set up Encore.ts in a Turborepo monorepo with shared packages that require building before use.
title: Turborepo
subtitle: Using Encore in a Turborepo monorepo
lang: ts
---

[Turborepo](https://turbo.build/repo) is a build system for JavaScript and TypeScript monorepos. This guide shows how to set up an Encore application within a Turborepo monorepo that depends on shared packages requiring compilation.

## Overview

When using Encore in a Turborepo monorepo, you may have shared packages (like utility libraries or shared types) that need to be built before the Encore app can use them. Since Encore parses your application on startup, these dependencies must be compiled first.

This guide covers two scenarios:
- **Local development**: Use Turborepo to build dependencies before running `encore run`
- **Deployment**: Use Encore's `prebuild` hook to automatically build dependencies when deploying via Encore Cloud or exporting a Docker image

## Project structure

A typical Turborepo setup with Encore looks like this:

```
my-turborepo/
├── apps/
│   └── backend/           # Encore application
│       ├── encore.app
│       ├── package.json
│       ├── tsconfig.json
│       └── article/
│           └── article.ts
├── packages/
│   └── shared/            # Shared library requiring build
│       ├── package.json
│       ├── tsconfig.json
│       ├── src/
│       │   └── index.ts
│       └── dist/          # Built output
│           └── index.js
├── turbo.json
├── package.json
└── package-lock.json
```

## Configuration

### Root package.json

Configure npm workspaces to include your apps and packages:

```json
{
  "name": "my-turborepo",
  "private": true,
  "packageManager": "npm@10.0.0",
  "scripts": {
    "build": "turbo run build",
    "dev": "turbo run dev"
  },
  "devDependencies": {
    "turbo": "^2.0.0",
    "typescript": "^5.0.0"
  },
  "workspaces": [
    "apps/*",
    "packages/*"
  ]
}
```

The `packageManager` field is required by Turborepo. Adjust the version to match your installed npm version (run `npm --version` to check).

### turbo.json

Configure Turborepo's build pipeline in the root `turbo.json`. The `@repo/backend#dev` task depends on the shared package being built first:

```json
{
  "$schema": "https://turbo.build/schema.json",
  "tasks": {
    "build": {
      "dependsOn": ["^build"],
      "outputs": ["dist/**"]
    },
    "@repo/backend#dev": {
      "dependsOn": ["@repo/shared#build"],
      "cache": false,
      "persistent": true
    },
    "dev": {
      "cache": false,
      "persistent": true
    }
  }
}
```

The `@repo/backend#dev` task configuration ensures the shared package is built before running `encore run` in local development.

### Shared package

Your shared package needs to compile TypeScript to JavaScript and expose the built output:

**packages/shared/package.json:**
```json
{
  "name": "@repo/shared",
  "version": "1.0.0",
  "private": true,
  "type": "module",
  "main": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "default": "./dist/index.js"
    }
  },
  "scripts": {
    "build": "tsc"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}
```

**packages/shared/tsconfig.json:**
```json
{
  "compilerOptions": {
    "outDir": "dist",
    "rootDir": "src",
    "moduleResolution": "bundler",
    "module": "ES2022",
    "target": "ES2022",
    "declaration": true
  },
  "include": ["src"],
  "exclude": ["node_modules", "dist"]
}
```

**packages/shared/src/index.ts:**
```ts
// Types shared between frontend and backend
export interface Article {
  slug: string;
  title: string;
  preview: string;
}

export interface CreateArticleRequest {
  title: string;
  content: string;
}

// Utility functions
export function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/\s+/g, "-");
}

export function truncate(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return text.slice(0, maxLength - 3) + "...";
}
```

### Encore application

The Encore app needs two key configurations:

1. **encore.app** - Use the `prebuild` hook to build dependencies during deployment
2. **package.json** - Declare the dependency on the shared package

To create the Encore app, run `encore app init --lang ts` from the `apps/backend` directory. Then add the `prebuild` hook to the generated `encore.app` file:

**apps/backend/encore.app:**
```json
{
    "id": "generated-id",
    "lang": "typescript",
    "build": {
        "hooks": {
            "prebuild": "npx turbo build --filter=@repo/backend^..."
        }
    }
}
```

The `prebuild` hook runs when deploying via Encore Cloud or when exporting a Docker image with the Encore CLI. The filter `@repo/backend^...` tells Turborepo to build all dependencies of `@repo/backend`. The `^` excludes the backend itself, building only its dependencies.

**apps/backend/package.json:**
```json
{
  "name": "@repo/backend",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "encore run"
  },
  "dependencies": {
    "@repo/shared": "*",
    "encore.dev": "latest"
  }
}
```

### Using the shared package

With this setup, you can import from your shared package in your Encore services:

**apps/backend/article/article.ts:**
```ts
import { api } from "encore.dev/api";
import type { Article, CreateArticleRequest } from "@repo/shared";
import { slugify, truncate } from "@repo/shared";

export const create = api(
  { expose: true, method: "POST", path: "/article" },
  async ({ title, content }: CreateArticleRequest): Promise<Article> => {
    return {
      slug: slugify(title),
      title: title,
      preview: truncate(content, 100),
    };
  },
);
```

## Running the application

### Installation

First, install all dependencies from the monorepo root:

```shell
$ npm install
```

This installs dependencies for all workspaces, including Turborepo.

### Local development

For local development, you need to build the shared packages before running `encore run`. From the monorepo root:

```shell
$ npx turbo run build
$ cd apps/backend && encore run
```

Or use Turborepo's `dev` task which handles the dependency ordering:

```shell
$ npx turbo run dev --filter=@repo/backend
```

The `turbo.json` configuration ensures `@repo/shared` is built before the backend's dev task runs.

### Deployment

When deploying via Encore Cloud or exporting a Docker image, the `prebuild` hook in `encore.app` automatically runs the Turborepo build pipeline.

<Callout type="info">

When deploying a monorepo to Encore Cloud, configure the root path to your Encore app in the app settings: **Settings > General > Root Directory** (e.g., `apps/backend`).

</Callout>

## Key points

- **Local development**: Run `npx turbo run build` before `encore run`, or use `npx turbo run dev --filter=@repo/backend` to handle dependency ordering automatically
- **Prebuild hook**: The `prebuild` hook in `encore.app` runs during deployment (Encore Cloud) or Docker export, not during local development
- **Turborepo filter**: Using `--filter=@repo/backend^...` builds only the dependencies of the backend (the `^` excludes the package itself)
