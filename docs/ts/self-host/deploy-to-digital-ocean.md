---
seotitle: How to deploy an Encore app to DigitalOcean
seodesc: Learn how to deploy an Encore application to DigitalOcean's App Platform using Docker.
title: Deploy to DigitalOcean
lang: ts
---

If you prefer manual deployment over the automation offered by Encore's Platform, Encore simplifies the process of deploying your app to the cloud provider of your choice. This guide will walk you through deploying an Encore app to DigitalOcean's App Platform using Docker.

### Video tutorial
<iframe width="560" height="315" src="https://www.youtube.com/embed/D3SjuCK_5qE?si=zLxEzG7dgTBlPkwU" title="Deploying a TypeScript backend to DigitalOcean using Docker & GitHub Actions" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe>

### Prerequisites
1. **DigitalOcean Account**: Make sure you have a DigitalOcean account. If not, you can [sign up here](https://www.digitalocean.com/).
2. **Docker Installed**: Ensure Docker is installed on your local machine. You can download it from the [Docker website](https://www.docker.com/get-started).
3. **Encore CLI**: Install the Encore CLI if you haven’t already. You can follow the installation instructions from the [Encore documentation](https://encore.dev/docs/ts/install).
4. **DigitalOcean CLI (Optional)**: You can install the DigitalOcean CLI for more flexibility and automation, but it’s not necessary for this tutorial.

### Step 1: Create an Encore App
1. **Create a New Encore App**: 
    - If you haven’t already, create a new Encore app using the Encore CLI.
    - You can use the following command to create a new app:
    ```bash
    encore app create myapp
    ``` 
    - Select the `Hello World` template.
    - Follow the prompts to create the app.

2. **Build a Docker image**:
    - Build the Encore app to generate the docker image for deployment:
    ```bash
    encore build docker myapp  
    ```
### Step 2: Push the Docker Image to a Container Registry
To deploy your Docker image to DigitalOcean, you need to push it to a container registry. DigitalOcean supports
its own container registry, but you can also use DockerHub or other registries. Here’s how to push the image to DigitalOcean’s registry:

1. **Create a DigitalOcean Container Registry**:
    - Go to the [DigitalOcean Control Panel](https://cloud.digitalocean.com/registries) and create a new container registry.
    - Follow the instructions to set it up.

2. **Login to DigitalOcean's registry**:
   Use the login command provided by DigitalOcean, which will look something like this:
   ```bash
   doctl registry login
   ```
   You’ll need the DigitalOcean CLI for this, which can be installed from [DigitalOcean CLI documentation](https://docs.digitalocean.com/reference/doctl/how-to/install/).

3. **Tag your Docker image**:
   Tag your image to match the registry’s URL.
   ```bash
   docker tag myapp registry.digitalocean.com/YOUR_REGISTRY_NAME/myapp:latest
   ```

4. **Push your Docker image to the registry**:
   ```bash
   docker push registry.digitalocean.com/YOUR_REGISTRY_NAME/myapp:latest
   ```

### Step 3: Deploy the Docker Image to DigitalOcean App Platform
1. **Navigate to the App Platform**:
   Go to [DigitalOcean's App Platform](https://cloud.digitalocean.com/apps).

2. **Create a New App**:
    - Click on **"Create App"**.
    - Choose the **"DigitalOcean Container Registry"** option.

3. **Select the Docker Image Source**:
    - Select the image you pushed earlier.

4. **Configure the App Settings**:
    - **Set up scaling options**: Configure the number of containers, CPU, and memory settings.
    - **Environment variables**: Add any environment variables your application might need.
    - **Choose the region**: Pick a region close to your users for better performance.

5. **Deploy the App**:
    - Click **"Next"**, review the settings, and click **"Create Resources"**.
    - DigitalOcean will take care of provisioning the infrastructure, pulling the Docker image, and starting the application.

### Step 4: Monitor and Manage the App
1. **Access the Application**:
    - Once deployed, you will get a public URL to access your application.
    - Test the app to ensure it’s running as expected, e.g. 
   ```bash
      curl https://myapp.ondigitalocean.app/hello/world
    ```

2. **View Logs and Metrics**:
    - Go to the **"Runtime Logs"** tab in the App Platform to view logs
    - Go to the **"Insights"** tab to view performance metrics.

3. **Manage Scaling and Deployment Settings**:
    - You can change the app configuration, such as scaling settings, deployment region, or environment variables.

### Step 5: Add a Database to Your App

DigitalOcean’s App Platform provides managed databases, allowing you to add a database to your app easily. Here’s how to set up a managed database for your app:

1. **Navigate to the DigitalOcean Control Panel**:
   - Go to [DigitalOcean Control Panel](https://cloud.digitalocean.com/).
   - Click on **"Databases"** in the left-hand sidebar.

2. **Create a New Database Cluster**:
   - Click **"Create Database Cluster"**.
   - Choose **PostgreSQL**
   - Select the **database version**, **data center region**, and **cluster configuration** (e.g., development or production settings based on your needs).
   - **Name the database** and configure other settings if necessary, then click **"Create Database Cluster"**.

3. **Configure the Database Settings**:
   - Once the database is created, go to the **"Connection Details"** tab of the database dashboard.
   - Copy the **connection string** or individual settings (host, port, username, password, database name). You will need these details to connect your app to the database.
   - Download the **CA certificate**

4. **Create a Database**
   - Connect to the database using the connection string provided by DigitalOcean.
   ```bash
   psql -h mydb.db.ondigitalocean.com -U doadmin -d mydb -p 25060
   ```
   - Create a database
   ```sql
    CREATE DATABASE mydb;
    ```
   - Create a table
   ```sql
     CREATE TABLE users (
        id SERIAL PRIMARY KEY,
        name VARCHAR(50)
     );
     INSERT INTO users (name) VALUES ('Alice');
   ```
   
5. **Declare a Database in your Encore app**:
   - Open your Encore app’s codebase.
   - Add `mydb` database to your app ([Encore Database Documentation](https://encore.dev/docs/ts/primitives/databases))
   ```typescript
      const mydb = new SQLDatabase("mydb", {
         migrations: "./migrations",
      });

      export const getUser = api(
        { expose: true, method: "GET", path: "/names/:id" },
        async ({id}: {id:number}): Promise<{ id: number; name: string }> => {
          return await mydb.queryRow`SELECT * FROM users WHERE id = ${id}` as { id: number; name: string };
        }
      );
   ```

6. **Create an Encore Infrastructure config**
   - Create a file named `infra.config.json` in the root of your Encore app.
   - Add the **CA certificate** and the connection details to the file:
   ```json
   {
      "$schema": "https://encore.dev/schemas/infra.schema.json",
      "sql_servers": [
      {
         "host": "mydb.db.ondigitalocean.com:25060",
         "tls_config": {
            "ca": "-----BEGIN CERTIFICATE-----\n..."
         },
         "databases": {
            "mydb": {
               "name": "mydb",
               "username": "doadmin",
               "password": {"$env": "DB_PASSWORD"}
             }
         }
      }]   
   }
   ```

7. **Set Up Environment Variables (Optional)**:
   - Go to the DigitalOcean App Platform dashboard.
   - Select your app.
   - In the **"Settings"** section, go to **"App-Level Environment Variables"**
   - Add the database password as an encrypted environment variable called `DB_PASSWORD`.

8. **Build and push the Docker image**:
   - Build the Docker image with the updated configuration.
   ```bash
   encore build docker --config infra.config.json myapp
   ```
   - Tag and push the Docker image to the DigitalOcean container registry.
   ```bash
   docker tag myapp registry.digitalocean.com/YOUR_REGISTRY_NAME/myapp:latest
   docker push registry.digitalocean.com/YOUR_REGISTRY_NAME/myapp:latest
   ```

9. **Test the Database Connection**:
   - Redeploy the app on DigitalOcean to apply the changes.
   - Test the database connection by calling the API
   ```bash
    curl https://myapp.ondigitalocean.app/names/1
   ```

### Troubleshooting Tips
- **Deployment Failures**: Check the build logs for any errors. Make sure the Docker image is correctly tagged and pushed to the registry.
- **App Not Accessible**: Verify that the correct port is exposed in the Dockerfile and the App Platform configuration.
- **Database Connection Issues**: Ensure the database connection details are correct and the database is accessible from the app.

### Conclusion
That’s it! You’ve successfully deployed an Encore app to DigitalOcean’s App Platform using Docker. You can now scale your app, monitor its performance, and manage it easily through the DigitalOcean dashboard. If you encounter any issues, refer to the DigitalOcean documentation or the Encore community for help. Happy coding!
