---
seotitle: Using Encore with Nx in a monorepo
seodesc: Learn how to set up Encore.ts in an Nx monorepo with shared packages that require building before use.
title: Nx
subtitle: Using Encore in an Nx monorepo
lang: ts
---

[Nx](https://nx.dev) is a build system for JavaScript and TypeScript monorepos. This guide shows how to set up an Encore application within an Nx monorepo that depends on shared packages requiring compilation.

## Overview

When using Encore in an Nx monorepo, you may have shared packages (like utility libraries or shared types) that need to be built before the Encore app can use them. Since Encore parses your application on startup, these dependencies must be compiled first.

This guide covers two scenarios:
- **Local development**: Use Nx to build dependencies before running `encore run`
- **Deployment**: Use Encore's `prebuild` hook to automatically build dependencies when deploying via Encore Cloud or exporting a Docker image

## Project structure

A typical Nx setup with Encore looks like this:

```
my-nx-workspace/
├── apps/
│   └── backend/           # Encore application
│       ├── encore.app
│       ├── package.json
│       ├── project.json
│       ├── tsconfig.json
│       └── article/
│           └── article.ts
├── packages/
│   └── shared/            # Shared library requiring build
│       ├── package.json
│       ├── project.json
│       ├── tsconfig.json
│       ├── src/
│       │   └── index.ts
│       └── dist/          # Built output
│           └── index.js
├── nx.json
├── package.json
└── package-lock.json
```

## Configuration

### Root package.json

Configure npm workspaces to include your apps and packages:

```json
{
  "name": "my-nx-workspace",
  "private": true,
  "scripts": {
    "build": "nx run-many -t build",
    "dev": "nx run-many -t dev"
  },
  "devDependencies": {
    "nx": "^21.0.0",
    "typescript": "^5.0.0"
  },
  "workspaces": [
    "apps/*",
    "packages/*"
  ]
}
```

### nx.json

Configure Nx's build pipeline in the root `nx.json`:

```json
{
  "$schema": "./node_modules/nx/schemas/nx-schema.json",
  "targetDefaults": {
    "build": {
      "dependsOn": ["^build"],
      "outputs": ["{projectRoot}/dist/**"],
      "cache": true
    },
    "dev": {
      "cache": false
    }
  }
}
```

The `"dependsOn": ["^build"]` configuration ensures that a project's dependencies are built before the project itself.

### Shared package

Your shared package needs to compile TypeScript to JavaScript and expose the built output.

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

**packages/shared/project.json:**
```json
{
  "name": "@repo/shared",
  "$schema": "../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "packages/shared/src",
  "projectType": "library",
  "targets": {
    "build": {
      "executor": "nx:run-commands",
      "options": {
        "command": "tsc",
        "cwd": "packages/shared"
      },
      "outputs": ["{projectRoot}/dist"]
    }
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

The Encore app needs three key configurations:

1. **encore.app** - Use the `prebuild` hook to build dependencies during deployment
2. **package.json** - Declare the dependency on the shared package
3. **project.json** - Configure Nx targets and task dependencies

To create the Encore app, run `encore app init --lang ts` from the `apps/backend` directory. Then add the `prebuild` hook to the generated `encore.app` file:

**apps/backend/encore.app:**
```json
{
    "id": "generated-id",
    "lang": "typescript",
    "build": {
        "hooks": {
            "prebuild": "npx nx build-deps @repo/backend"
        }
    }
}
```

The `prebuild` hook runs when deploying via Encore Cloud or when exporting a Docker image with the Encore CLI. The `build-deps` target builds all dependencies of the backend.

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

**apps/backend/project.json:**
```json
{
  "name": "@repo/backend",
  "$schema": "../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "apps/backend",
  "projectType": "application",
  "targets": {
    "dev": {
      "executor": "nx:run-commands",
      "options": {
        "command": "encore run",
        "cwd": "apps/backend"
      },
      "dependsOn": ["^build"],
      "cache": false
    },
    "build-deps": {
      "dependsOn": ["^build"],
      "cache": true
    }
  }
}
```

The `"dependsOn": ["^build"]` configuration uses the `^` prefix to indicate "run the build target on all dependencies first". This automatically builds all shared packages before running `encore run`, without needing to list each dependency explicitly.

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

This installs dependencies for all workspaces, including Nx.

### Local development

For local development, you need to build the shared packages before running `encore run`. From the monorepo root:

```shell
$ npx nx build-deps @repo/backend
$ cd apps/backend && encore run
```

Or use Nx's `dev` target which handles the dependency ordering:

```shell
$ npx nx dev @repo/backend
```

The `"dependsOn": ["^build"]` configuration ensures all dependencies are built before the backend's dev target runs.

### Deployment

When deploying via Encore Cloud or exporting a Docker image, the `prebuild` hook in `encore.app` automatically runs the Nx build.

<Callout type="info">

When deploying a monorepo to Encore Cloud, configure the root path to your Encore app in the app settings: **Settings > General > Root Directory** (e.g., `apps/backend`).

</Callout>

## Key points

- **Local development**: Run `npx nx build-deps @repo/backend` before `encore run`, or use `npx nx dev @repo/backend` to handle dependency ordering automatically
- **Prebuild hook**: The `prebuild` hook in `encore.app` runs during deployment (Encore Cloud) or Docker export, not during local development
- **Task dependencies**: Use `"dependsOn": ["^build"]` in `project.json` to automatically build all dependencies before running a target
