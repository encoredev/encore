---
seotitle: How to deploy an Encore app to Railway
seodesc: Learn how to deploy an Encore application to Railway using Docker and GitHub Actions.
title: Deploy to Railway
lang: ts
---

If you prefer manual deployment over the automation offered by Encore's Platform, Encore simplifies the process of deploying your app to the cloud provider of your choice. This guide will walk you through deploying an Encore app to Railway using Docker through GitHub Actions.

### Prerequisites
1. **Railway Account**: Make sure you have a Railway account. If not, you can [sign up here](https://railway.com/).
2. **Docker Installed**: Ensure Docker is installed on your local machine, Docker is used by Encore to run databases locally. You can download it from the [Docker website](https://www.docker.com/get-started).
3. **Encore CLI**: Install the Encore CLI if you haven’t already. You can follow the installation instructions from the [Encore documentation](https://encore.dev/docs/ts/install).

### Step 1: Create an Encore App and a GitHub repository
1. **Create a New Encore App**: 
    - Create a new Encore app using the Encore CLI by running the following command:
    ```bash
    encore app create
    ``` 
    - Select the `Hello World` template.
    - Follow the prompts to create the app.

2. **Push the code to a GitHub repo**:
    - Create a new repo (public or private) on GitHub and push the code to it.
   
### Step 2: Push the Docker Image to GitHub's Container Registry
To deploy your Docker image to Railway, you first need to push it to a container registry. We will be using GitHub's container registry, but you can also use DockerHub or other registries. 
Instead of pushing the image manually we will be using GitHub actions to automate the process.

1. **Create a GitHub Actions YAML file**:
   - In your repo, create a `.github/workflows/deploy-image-yaml` file with the following contents:
   
```yaml
name: Build, Push and Deploy a Docker Image to Railway

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

      - name: Log in to the Container registry
        uses: docker/login-action@v3.3.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Download Encore CLI script
        uses: sozo-design/curl@v1.0.2
        with:
          args: --output install.sh -L https://encore.dev/install.sh

      - name: Install Encore CLI
        run: bash install.sh

      - name: Build Docker image
        run: /home/runner/.encore/bin/encore build --config railway-infra.config.json docker myapp

      - name: Tag Docker image
        run: docker tag myapp ghcr.io/${{ github.repository }}:latest

      - name: Push Docker image
        run: docker push ghcr.io/${{ github.repository }}:latest
```

This will install the Encore CLI, build the Docker image, tag it, and push it to GitHub's container registry everytime you push to the `main` branch.
The dynamic values like `${{ github.repository }}` will be filled in automatically by GitHub, you should not need to do anything.

2. **Add, commit and push the changes**:
   - Push the changes to your GitHub repository to trigger the GitHub action.

### Step 3: Deploy the Docker Image to Railway

1. **Create a new Project on Railway**:
    - Log in to Railway and go to your dashboard. 
    - Click on **"New"**.
    - Choose the **"Empty project"** option.

2. **Create a new service inside your new project**:
    - Click on **"Create"**. 
    - Select the "Docker Image" option.
    - Enter the Docker Image URI, should be something like `ghcr.io/username/repo:latest`. You can should be able to find the Docker Image under **Packages** in your GitHub repo.
    - Deploy the service.

3. **Expose the service**:
    - Click on the tne newly created service.
    - Go to the **"Settings"** tab.
    - Click on **"Generate Domain"**.
    - Select `8080` as the port.
    - Click on **"Generate"**.

4. **Access the application**:
    - Once deployed, and exposed you will get a public URL to access your application. It should look something like this: `https://repo-name-production.up.railway.app/`.
    - Test the app to ensure it's running as expected, e.g. 
   ```bash
      curl https://repo-name-production.up.railway.app/hello/world
    ```

### Step 4: Automate the Deployment Process
Railway has no way of knowing that you've pushed a new image to the container registry, but we can use Railway's GraphQL API to trigger a new deployment whenever a new image is pushed to the registry.

1. **Generate a Railway API Token**:
   - Go to your Railway dashboard.
   - Click on your profile icon in the top right corner.
   - Go to **"Account Settings"**.
   - Click on **"Tokens"**.
   - Give the token a name and click on **"Create"**.
   - Copy the generated token.

2. **Add the Railway API Token to GitHub Secrets**:
   - Go to your GitHub repository.
   - Go to **"Settings"**.
   - Click on **"Secrets and variables" → "Actions"**.
   - Click on **"New repository secret"**.
   - Add a new secret called `RAILWAY_API_TOKEN` and paste the token you copied earlier.

3. **Add a JavaScript script to your repo**:
   - Create a new file in your repo named `script.js` with the following contents:
```javascript
const TOKEN = process.argv.slice(2)[0];
const ENVIRONMENT_ID = "<your environment id>"
const SERVICE_ID = "<your service id>"

const resp = await fetch('https://backboard.railway.com/graphql/v2', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'authorization': `Bearer ${TOKEN}`,
  },
  body: JSON.stringify({
    query: `
      mutation ServiceInstanceRedeploy {
          serviceInstanceRedeploy(
              environmentId: "${ENVIRONMENT_ID}"
              serviceId: "${SERVICE_ID}"
          )
      }`
  }),
})

const data = await resp.json()

if (data.errors) {
  console.error(data.errors)
  throw new Error('Failed to redeploy service')
}

console.log(data)
 ```
   - Replace `<your environment id>` and `<your service id>` with the actual values. You can find these values in the Railway dashboard URL when you're on the service page.

4. **Add new steps to the GitHub Actions YAML file**:
   - At the bottom of the existing file, add the following steps to call the script:
```yaml
      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: 22

      - name: Trigger Railway deployment
        run: node script.js ${{ secrets.RAILWAY_API_TOKEN }}
```

Whenever you push a new Docker Image to the container registry, the GitHub action will trigger a new deployment on Railway.

### Conclusion
That’s it! You’ve successfully deployed an Encore app to Railway using Docker. You can now scale your app, monitor its performance, and manage it easily through the Railway dashboard. If you encounter any issues, refer to the Railway documentation or the Encore community for help. Happy coding!
