---
seotitle: Using Sequelize with Encore
seodesc: Learn how to use Sequelize with Encore to interact with SQL databases.
title: Using Sequelize with Encore
lang: ts
---

Here is an example of using [Sequelize](https://sequelize.org/) with Encore.ts. We use `DB.connectionString` supply the connection string to sequelize:

```ts
-- database.ts --
import {
  Model,
  InferAttributes,
  InferCreationAttributes,
  CreationOptional,
  DataTypes,
  Sequelize,
} from "sequelize";
import { SQLDatabase } from "encore.dev/storage/sqldb";

const DB = new SQLDatabase('encore_sequelize_test', {
  migrations: './migrations',
});

const sequelize = new Sequelize(DB.connectionString);

class User extends Model<InferAttributes<User>, InferCreationAttributes<User>> {
  declare id: CreationOptional<number>;
  declare name: string;
  declare surname: string;
}

const count = await User.count();

-- migrations/1_create_user.up.sql --
CREATE TABLE "user" (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  surname TEXT NOT NULL
);
```

<GitHubLink
href="https://github.com/encoredev/examples/tree/main/ts/sequelize"
desc="Using Sequelize ORM with Encore.ts"
/>