---
seotitle: Integrate with your CI/CD pipeline
seodesc: Learn how to integrate Encore.ts with your CI/CD pipeline.
title: Integrate with your CI/CD pipeline
lang: ts
---

Encore seamlessly integrates with any CI/CD pipeline through its CLI tools. You can automate Docker image creation using the `encore build` command as part of your deployment workflow.

## Integrating with CI/CD Platforms

While every CI/CD pipeline is unique, integrating Encore follows a straightforward process. Here are the key steps:

1. Install the Encore CLI in your CI environment
2. Use `encore build docker` to create Docker images
3. Push the images to your container registry
4. Deploy to your infrastructure

Refer to your CI/CD platform's documentation for more details on how to integrate CLI tools like `encore build`.

### GitHub actions example

This example shows how to build, push, and deploy an Encore Docker image to DigitalOcean using GitHub Actions.
The DigitalOcean application is set up re-deploy the application every time an image with the tag `latest` is uploaded. 

```yaml
name: Build, Push and Deploy a Encore Docker Image to DigitalOcean

on:
  push:
    branches: [ main ]

permissions:
  contents: read
  packages: write

jobs:
  build-push-deploy-image:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Download Encore CLI script
        uses: sozo-design/curl@v1.0.2
        with:
          args: --output install.sh -L https://encore.dev/install.sh

      - name: Install Encore CLI
        run: bash install.sh

      - name: Log in to DigitalOcean container registry
        run: docker login registry.digitalocean.com -u my-email@gmail.com -p ${{ secrets.DIGITALOCEAN_ACCESS_TOKEN }}

      - name: Build Docker image
        run: /home/runner/.encore/bin/encore build docker myapp

      - name: Tag Docker image
        run: docker tag myapp registry.digitalocean.com/<YOUR_CONTAINER_REGISTRY_NAME>/<YOUR_IMAGE_REPOSITORY_NAME>:latest

      - name: Push Docker image
        run: docker push registry.digitalocean.com/<YOUR_CONTAINER_REGISTRY_NAME>/<YOUR_IMAGE_REPOSITORY_NAME>:latest
```

## Building Docker Images

The `encore build docker` command provides several options to customize your builds:

```bash
# Build specific services and gateways
encore build docker --services=service1,service2 --gateways=api-gateway MY-IMAGE:TAG

# Customize the base image
encore build docker --base=node:18-alpine MY-IMAGE:TAG
```

The image will default to run on port 8080, but you can customize it by setting the `PORT` environment variable when starting your image.

```bash
docker run -e PORT=8081 -p 8081:8081 MY-IMAGE:TAG
```

Learn more about the `encore build docker` command in the [build Docker images](/docs/ts/self-host/build) guide.

Continue to learn how to [configure infrastructure](/docs/ts/self-host/configure-infra).
