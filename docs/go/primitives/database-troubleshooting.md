---
seotitle: Troubleshooting SQL databases
seodesc: Advice on troubleshooting SQL databases in Encore.go
title: Troubleshooting Databases
subtitle: Advice on troubleshooting SQL databases in Encore.go
infobox: {
  title: "SQL Databases",
  import: "encore.dev/storage/sqldb"
}
lang: go
---

When you run your application locally with `encore run`, Encore provisions local databases using [Docker](https://docker.com). If this fails with a database error, it can often be resolved by making sure you have Docker installed and running, or by restarting the Encore daemon using `encore daemon`.

If this does not resolve the issue, here are steps to resolve common errors:

** Error: sqldb: unknown database **

This error is often caused by a problem with the initial migration file, such as incorrect naming or location.

- Verify that you've [created the migration file](/docs/go/primitives/databases#defining-a-database-schema) correctly, then try `encore run` again.

** Error: could not connect to the database **

When you can't connect to the database in your local environment, there's likely an issue with Docker:

- Make sure that you have [Docker](https://docker.com) installed and running, then try `encore run` again.
- If this fails, restart the Encore daemon by running `encore daemon`, then try `encore run` again.

** Error: Creating PostgreSQL database cluster Failed **

This means Encore was not able to create the database. Often this is due to a problem with Docker.

- Check if you have permission to access Docker by running `docker images`.
- Set the correct permissions with `sudo usermod -aG docker $USER` (Learn more in the [Docker documentation](https://docs.docker.com/engine/install/linux-postinstall/))
- Then log out and log back in so that your group membership is refreshed.

** Error: unable to add CA to cert pool **

This error is commonly caused by the presence of the file `$HOME/.postgresql/root.crt` on the filesystem.
When this file is present the PostgreSQL client library will assume the database server has that root certificate,
which will cause the above error.

- Remove or rename the file, then try `encore run` again.
