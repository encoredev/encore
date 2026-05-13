---
title: encore.dev/storage/sqldb
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Connection -->
### Connection

<!-- source: storage/sqldb/database.ts:360 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L360)

Represents a dedicated connection to a database.

#### Extends

- `BaseQueryExecutor`

#### Constructors

##### Constructor

`new Connection(impl): Connection`

<!-- source: storage/sqldb/database.ts:363 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L363)

###### Parameters

###### impl

`SQLConn`

###### Returns

[`Connection`](#connection)

###### Overrides

`BaseQueryExecutor.constructor`

#### Properties

##### impl

`protected readonly impl: SQLConn`

###### Overrides

`BaseQueryExecutor.impl`

#### Methods

##### close()

`close(): Promise<void>`

<!-- source: storage/sqldb/database.ts:370 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L370)

Returns the connection to the database pool.

###### Returns

`Promise`\<`void`\>

##### exec()

`exec(strings, ...params): Promise<void>`

<!-- source: storage/sqldb/database.ts:229 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L229)

exec executes a query without returning any rows.

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`void`\>

###### Example

```ts
const email = "foo@example.com";
const result = database.exec`DELETE FROM users WHERE email=${email}`
```

###### Inherited from

`BaseQueryExecutor.exec`

##### query()

`query<T>(strings, ...params): AsyncGenerator<T>`

<!-- source: storage/sqldb/database.ts:69 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L69)

query queries the database using a template string, replacing your placeholders in the template
with parametrised values without risking SQL injections.

It returns an async generator, that allows iterating over the results
in a streaming fashion using `for await`.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`AsyncGenerator`\<`T`\>

###### Example

```ts
const email = "foo@example.com";
const result = database.query`SELECT id FROM users WHERE email=${email}`

This produces the query: "SELECT id FROM users WHERE email=$1".
```

###### Inherited from

`BaseQueryExecutor.query`

##### queryAll()

`queryAll<T>(strings, ...params): Promise<T[]>`

<!-- source: storage/sqldb/database.ts:129 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L129)

queryAll queries the database using a template string, replacing your placeholders in the template
with parametrised values without risking SQL injections.

It returns an array of all results.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`T`[]\>

###### Example

```ts
const email = "foo@example.com";
const result = database.queryAll`SELECT id FROM users WHERE email=${email}`

This produces the query: "SELECT id FROM users WHERE email=$1".
```

###### Inherited from

`BaseQueryExecutor.queryAll`

##### queryRow()

`queryRow<T>(strings, ...params): Promise<T | null>`

<!-- source: storage/sqldb/database.ts:184 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L184)

queryRow is like query but returns only a single row.
If the query selects no rows it returns null.
Otherwise it returns the first row and discards the rest.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`T` \| `null`\>

###### Example

```ts
const email = "foo@example.com";
const result = database.queryRow`SELECT id FROM users WHERE email=${email}`
```

###### Inherited from

`BaseQueryExecutor.queryRow`

##### rawExec()

`rawExec(query, ...params): Promise<void>`

<!-- source: storage/sqldb/database.ts:255 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L255)

rawExec executes a query without returning any rows.

###### Parameters

###### query

`string`

The raw SQL query string.

###### params

...[`Primitive`](#primitive)[]

The parameters to be used in the query.

###### Returns

`Promise`\<`void`\>

A promise that resolves when the query has been executed.

###### Example

```ts
const query = "DELETE FROM users WHERE email=$1";
const email = "foo@example.com";
await database.rawExec(query, email);
```

###### Inherited from

`BaseQueryExecutor.rawExec`

##### rawQuery()

`rawQuery<T>(query, ...params): AsyncGenerator<T>`

<!-- source: storage/sqldb/database.ts:101 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L101)

rawQuery queries the database using a raw parametrised SQL query and parameters.

It returns an async generator, that allows iterating over the results
in a streaming fashion using `for await`.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### query

`string`

The raw SQL query string.

###### params

...[`Primitive`](#primitive)[]

The parameters to be used in the query.

###### Returns

`AsyncGenerator`\<`T`\>

An async generator that yields rows from the query result.

###### Example

```ts
const query = "SELECT id FROM users WHERE email=$1";
const email = "foo@example.com";
for await (const row of database.rawQuery(query, email)) {
  console.log(row);
}
```

###### Inherited from

`BaseQueryExecutor.rawQuery`

##### rawQueryAll()

`rawQueryAll<T>(query, ...params): Promise<T[]>`

<!-- source: storage/sqldb/database.ts:158 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L158)

rawQueryAll queries the database using a raw parametrised SQL query and parameters.

It returns an array of all results.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### query

`string`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`T`[]\>

###### Example

```ts
const query = "SELECT id FROM users WHERE email=$1";
const email = "foo@example.com";
const rows = await database.rawQueryAll(query, email);
```

###### Inherited from

`BaseQueryExecutor.rawQueryAll`

##### rawQueryRow()

`rawQueryRow<T>(query, ...params): Promise<T | null>`

<!-- source: storage/sqldb/database.ts:211 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L211)

rawQueryRow is like rawQuery but returns only a single row.
If the query selects no rows, it returns null.
Otherwise, it returns the first row and discards the rest.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### query

`string`

The raw SQL query string.

###### params

...[`Primitive`](#primitive)[]

The parameters to be used in the query.

###### Returns

`Promise`\<`T` \| `null`\>

A promise that resolves to a single row or null.

###### Example

```ts
const query = "SELECT id FROM users WHERE email=$1";
const email = "foo@example.com";
const result = await database.rawQueryRow(query, email);
console.log(result);
```

###### Inherited from

`BaseQueryExecutor.rawQueryRow`

***

<!-- symbol-end -->

<!-- symbol-start: SQLDatabase -->
### SQLDatabase

<!-- source: storage/sqldb/database.ts:273 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L273)

Constructing a new database object will result in Encore provisioning a database with
that name and returning this object to represent it.

If you want to reference an existing database, use `Database.Named(name)` as it is a
compile error to create duplicate databases.

#### Extends

- `BaseQueryExecutor`

#### Constructors

##### Constructor

`new SQLDatabase(name, cfg?): SQLDatabase`

<!-- source: storage/sqldb/database.ts:276 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L276)

###### Parameters

###### name

`string`

###### cfg?

[`SQLDatabaseConfig`](#sqldatabaseconfig)

###### Returns

[`SQLDatabase`](#sqldatabase)

###### Overrides

`BaseQueryExecutor.constructor`

#### Properties

##### impl

`protected readonly impl: SQLDatabase`

###### Overrides

`BaseQueryExecutor.impl`

#### Accessors

##### connectionString

###### Get Signature

`get connectionString(): string`

Returns the connection string for the database

###### Returns

`string`

#### Methods

##### acquire()

`acquire(): Promise<Connection>`

<!-- source: storage/sqldb/database.ts:301 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L301)

Acquires a database connection from the database pool.

When the connection is closed or is garbage-collected, it is returned to the pool.

###### Returns

`Promise`\<[`Connection`](#connection)\>

a new connection to the database

##### begin()

`begin(): Promise<Transaction>`

<!-- source: storage/sqldb/database.ts:312 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L312)

Begins a database transaction.

Make sure to always call `rollback` or `commit` to prevent hanging transactions.

###### Returns

`Promise`\<[`Transaction`](#transaction)\>

a transaction object that implements AsycDisposable

##### exec()

`exec(strings, ...params): Promise<void>`

<!-- source: storage/sqldb/database.ts:229 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L229)

exec executes a query without returning any rows.

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`void`\>

###### Example

```ts
const email = "foo@example.com";
const result = database.exec`DELETE FROM users WHERE email=${email}`
```

###### Inherited from

`BaseQueryExecutor.exec`

##### query()

`query<T>(strings, ...params): AsyncGenerator<T>`

<!-- source: storage/sqldb/database.ts:69 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L69)

query queries the database using a template string, replacing your placeholders in the template
with parametrised values without risking SQL injections.

It returns an async generator, that allows iterating over the results
in a streaming fashion using `for await`.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`AsyncGenerator`\<`T`\>

###### Example

```ts
const email = "foo@example.com";
const result = database.query`SELECT id FROM users WHERE email=${email}`

This produces the query: "SELECT id FROM users WHERE email=$1".
```

###### Inherited from

`BaseQueryExecutor.query`

##### queryAll()

`queryAll<T>(strings, ...params): Promise<T[]>`

<!-- source: storage/sqldb/database.ts:129 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L129)

queryAll queries the database using a template string, replacing your placeholders in the template
with parametrised values without risking SQL injections.

It returns an array of all results.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`T`[]\>

###### Example

```ts
const email = "foo@example.com";
const result = database.queryAll`SELECT id FROM users WHERE email=${email}`

This produces the query: "SELECT id FROM users WHERE email=$1".
```

###### Inherited from

`BaseQueryExecutor.queryAll`

##### queryRow()

`queryRow<T>(strings, ...params): Promise<T | null>`

<!-- source: storage/sqldb/database.ts:184 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L184)

queryRow is like query but returns only a single row.
If the query selects no rows it returns null.
Otherwise it returns the first row and discards the rest.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`T` \| `null`\>

###### Example

```ts
const email = "foo@example.com";
const result = database.queryRow`SELECT id FROM users WHERE email=${email}`
```

###### Inherited from

`BaseQueryExecutor.queryRow`

##### rawExec()

`rawExec(query, ...params): Promise<void>`

<!-- source: storage/sqldb/database.ts:255 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L255)

rawExec executes a query without returning any rows.

###### Parameters

###### query

`string`

The raw SQL query string.

###### params

...[`Primitive`](#primitive)[]

The parameters to be used in the query.

###### Returns

`Promise`\<`void`\>

A promise that resolves when the query has been executed.

###### Example

```ts
const query = "DELETE FROM users WHERE email=$1";
const email = "foo@example.com";
await database.rawExec(query, email);
```

###### Inherited from

`BaseQueryExecutor.rawExec`

##### rawQuery()

`rawQuery<T>(query, ...params): AsyncGenerator<T>`

<!-- source: storage/sqldb/database.ts:101 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L101)

rawQuery queries the database using a raw parametrised SQL query and parameters.

It returns an async generator, that allows iterating over the results
in a streaming fashion using `for await`.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### query

`string`

The raw SQL query string.

###### params

...[`Primitive`](#primitive)[]

The parameters to be used in the query.

###### Returns

`AsyncGenerator`\<`T`\>

An async generator that yields rows from the query result.

###### Example

```ts
const query = "SELECT id FROM users WHERE email=$1";
const email = "foo@example.com";
for await (const row of database.rawQuery(query, email)) {
  console.log(row);
}
```

###### Inherited from

`BaseQueryExecutor.rawQuery`

##### rawQueryAll()

`rawQueryAll<T>(query, ...params): Promise<T[]>`

<!-- source: storage/sqldb/database.ts:158 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L158)

rawQueryAll queries the database using a raw parametrised SQL query and parameters.

It returns an array of all results.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### query

`string`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`T`[]\>

###### Example

```ts
const query = "SELECT id FROM users WHERE email=$1";
const email = "foo@example.com";
const rows = await database.rawQueryAll(query, email);
```

###### Inherited from

`BaseQueryExecutor.rawQueryAll`

##### rawQueryRow()

`rawQueryRow<T>(query, ...params): Promise<T | null>`

<!-- source: storage/sqldb/database.ts:211 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L211)

rawQueryRow is like rawQuery but returns only a single row.
If the query selects no rows, it returns null.
Otherwise, it returns the first row and discards the rest.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### query

`string`

The raw SQL query string.

###### params

...[`Primitive`](#primitive)[]

The parameters to be used in the query.

###### Returns

`Promise`\<`T` \| `null`\>

A promise that resolves to a single row or null.

###### Example

```ts
const query = "SELECT id FROM users WHERE email=$1";
const email = "foo@example.com";
const result = await database.rawQueryRow(query, email);
console.log(result);
```

###### Inherited from

`BaseQueryExecutor.rawQueryRow`

##### named()

`static named<name>(name): SQLDatabase`

<!-- source: storage/sqldb/database.ts:284 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L284)

Reference an existing database by name, if the database doesn't
exist yet, use `new Database(name)` instead.

###### Type Parameters

###### name

`name` *extends* `string`

###### Parameters

###### name

`StringLiteral`\<`name`\>

###### Returns

[`SQLDatabase`](#sqldatabase)

***

<!-- symbol-end -->

<!-- symbol-start: Transaction -->
### Transaction

<!-- source: storage/sqldb/database.ts:324 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L324)

Represents a database transaction.

Make sure to always call `rollback` or `commit` to prevent hanging transactions.

#### Extends

- `BaseQueryExecutor`

#### Implements

- `AsyncDisposable`

#### Constructors

##### Constructor

`new Transaction(impl): Transaction`

<!-- source: storage/sqldb/database.ts:328 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L328)

###### Parameters

###### impl

`Transaction`

###### Returns

[`Transaction`](#transaction)

###### Overrides

`BaseQueryExecutor.constructor`

#### Properties

##### impl

`protected readonly impl: Transaction`

###### Overrides

`BaseQueryExecutor.impl`

#### Methods

##### \[asyncDispose\]()

`asyncDispose: Promise<void>`

<!-- source: storage/sqldb/database.ts:350 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L350)

###### Returns

`Promise`\<`void`\>

###### Implementation of

`AsyncDisposable.[asyncDispose]`

##### commit()

`commit(): Promise<void>`

<!-- source: storage/sqldb/database.ts:335 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L335)

Commit the transaction.

###### Returns

`Promise`\<`void`\>

##### exec()

`exec(strings, ...params): Promise<void>`

<!-- source: storage/sqldb/database.ts:229 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L229)

exec executes a query without returning any rows.

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`void`\>

###### Example

```ts
const email = "foo@example.com";
const result = database.exec`DELETE FROM users WHERE email=${email}`
```

###### Inherited from

`BaseQueryExecutor.exec`

##### query()

`query<T>(strings, ...params): AsyncGenerator<T>`

<!-- source: storage/sqldb/database.ts:69 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L69)

query queries the database using a template string, replacing your placeholders in the template
with parametrised values without risking SQL injections.

It returns an async generator, that allows iterating over the results
in a streaming fashion using `for await`.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`AsyncGenerator`\<`T`\>

###### Example

```ts
const email = "foo@example.com";
const result = database.query`SELECT id FROM users WHERE email=${email}`

This produces the query: "SELECT id FROM users WHERE email=$1".
```

###### Inherited from

`BaseQueryExecutor.query`

##### queryAll()

`queryAll<T>(strings, ...params): Promise<T[]>`

<!-- source: storage/sqldb/database.ts:129 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L129)

queryAll queries the database using a template string, replacing your placeholders in the template
with parametrised values without risking SQL injections.

It returns an array of all results.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`T`[]\>

###### Example

```ts
const email = "foo@example.com";
const result = database.queryAll`SELECT id FROM users WHERE email=${email}`

This produces the query: "SELECT id FROM users WHERE email=$1".
```

###### Inherited from

`BaseQueryExecutor.queryAll`

##### queryRow()

`queryRow<T>(strings, ...params): Promise<T | null>`

<!-- source: storage/sqldb/database.ts:184 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L184)

queryRow is like query but returns only a single row.
If the query selects no rows it returns null.
Otherwise it returns the first row and discards the rest.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### strings

`TemplateStringsArray`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`T` \| `null`\>

###### Example

```ts
const email = "foo@example.com";
const result = database.queryRow`SELECT id FROM users WHERE email=${email}`
```

###### Inherited from

`BaseQueryExecutor.queryRow`

##### rawExec()

`rawExec(query, ...params): Promise<void>`

<!-- source: storage/sqldb/database.ts:255 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L255)

rawExec executes a query without returning any rows.

###### Parameters

###### query

`string`

The raw SQL query string.

###### params

...[`Primitive`](#primitive)[]

The parameters to be used in the query.

###### Returns

`Promise`\<`void`\>

A promise that resolves when the query has been executed.

###### Example

```ts
const query = "DELETE FROM users WHERE email=$1";
const email = "foo@example.com";
await database.rawExec(query, email);
```

###### Inherited from

`BaseQueryExecutor.rawExec`

##### rawQuery()

`rawQuery<T>(query, ...params): AsyncGenerator<T>`

<!-- source: storage/sqldb/database.ts:101 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L101)

rawQuery queries the database using a raw parametrised SQL query and parameters.

It returns an async generator, that allows iterating over the results
in a streaming fashion using `for await`.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### query

`string`

The raw SQL query string.

###### params

...[`Primitive`](#primitive)[]

The parameters to be used in the query.

###### Returns

`AsyncGenerator`\<`T`\>

An async generator that yields rows from the query result.

###### Example

```ts
const query = "SELECT id FROM users WHERE email=$1";
const email = "foo@example.com";
for await (const row of database.rawQuery(query, email)) {
  console.log(row);
}
```

###### Inherited from

`BaseQueryExecutor.rawQuery`

##### rawQueryAll()

`rawQueryAll<T>(query, ...params): Promise<T[]>`

<!-- source: storage/sqldb/database.ts:158 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L158)

rawQueryAll queries the database using a raw parametrised SQL query and parameters.

It returns an array of all results.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### query

`string`

###### params

...[`Primitive`](#primitive)[]

###### Returns

`Promise`\<`T`[]\>

###### Example

```ts
const query = "SELECT id FROM users WHERE email=$1";
const email = "foo@example.com";
const rows = await database.rawQueryAll(query, email);
```

###### Inherited from

`BaseQueryExecutor.rawQueryAll`

##### rawQueryRow()

`rawQueryRow<T>(query, ...params): Promise<T | null>`

<!-- source: storage/sqldb/database.ts:211 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L211)

rawQueryRow is like rawQuery but returns only a single row.
If the query selects no rows, it returns null.
Otherwise, it returns the first row and discards the rest.

###### Type Parameters

###### T

`T` *extends* [`ResultRow`](#resultrow) = `Record`\<`string`, `any`\>

###### Parameters

###### query

`string`

The raw SQL query string.

###### params

...[`Primitive`](#primitive)[]

The parameters to be used in the query.

###### Returns

`Promise`\<`T` \| `null`\>

A promise that resolves to a single row or null.

###### Example

```ts
const query = "SELECT id FROM users WHERE email=$1";
const email = "foo@example.com";
const result = await database.rawQueryRow(query, email);
console.log(result);
```

###### Inherited from

`BaseQueryExecutor.rawQueryRow`

##### rollback()

`rollback(): Promise<void>`

<!-- source: storage/sqldb/database.ts:344 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L344)

Rollback the transaction.

###### Returns

`Promise`\<`void`\>

<!-- symbol-end -->

## Interfaces

<!-- symbol-start: SQLDatabaseConfig -->
### SQLDatabaseConfig

<!-- source: storage/sqldb/database.ts:15 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L15)

Configuration for a `SQLDatabase`.

#### Properties

##### migrations?

`optional migrations?: string | SQLMigrationsConfig`

***

<!-- symbol-end -->

<!-- symbol-start: SQLMigrationsConfig -->
### SQLMigrationsConfig

<!-- source: storage/sqldb/database.ts:8 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L8)

Configures how database migrations are managed for a `SQLDatabase`.

#### Properties

##### path

`path: string`

##### source?

`optional source?: "prisma" | "drizzle" | "drizzle/v1"`

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: Primitive -->
### Primitive

```ts
type Primitive = 
  | string
  | string[]
  | number
  | number[]
  | boolean
  | boolean[]
  | Buffer
  | Date
  | Date[]
  | Record<string, any>
  | Record<string, any>[]
  | BigInt
  | BigInt[]
  | null
  | undefined;
```

<!-- source: storage/sqldb/database.ts:28 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L28)

Represents a type that can be used in query template literals

***

<!-- symbol-end -->

<!-- symbol-start: ResultRow -->
### ResultRow

`type ResultRow = Record<string, any>`

<!-- source: storage/sqldb/database.ts:25 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L25)

Represents a single row from a query result


<!-- symbol-end -->