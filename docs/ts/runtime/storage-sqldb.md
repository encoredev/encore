---
title: encore.dev/storage/sqldb
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Connection -->
### Connection <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L360" target="_blank" rel="noopener">source</a>

Represents a dedicated connection to a database.

#### Extends

- `BaseQueryExecutor`

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L363" target="_blank" rel="noopener">source</a>

`new Connection(impl): Connection`

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

##### close() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L370" target="_blank" rel="noopener">source</a>

`close(): Promise<void>`

Returns the connection to the database pool.

###### Returns

`Promise`\<`void`\>

##### exec() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L229" target="_blank" rel="noopener">source</a>

`exec(strings, ...params): Promise<void>`

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

##### query() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L69" target="_blank" rel="noopener">source</a>

`query<T>(strings, ...params): AsyncGenerator<T>`

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

##### queryAll() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L129" target="_blank" rel="noopener">source</a>

`queryAll<T>(strings, ...params): Promise<T[]>`

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

##### queryRow() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L184" target="_blank" rel="noopener">source</a>

`queryRow<T>(strings, ...params): Promise<T | null>`

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

##### rawExec() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L255" target="_blank" rel="noopener">source</a>

`rawExec(query, ...params): Promise<void>`

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

##### rawQuery() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L101" target="_blank" rel="noopener">source</a>

`rawQuery<T>(query, ...params): AsyncGenerator<T>`

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

##### rawQueryAll() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L158" target="_blank" rel="noopener">source</a>

`rawQueryAll<T>(query, ...params): Promise<T[]>`

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

##### rawQueryRow() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L211" target="_blank" rel="noopener">source</a>

`rawQueryRow<T>(query, ...params): Promise<T | null>`

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
### SQLDatabase <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L273" target="_blank" rel="noopener">source</a>

Constructing a new database object will result in Encore provisioning a database with
that name and returning this object to represent it.

If you want to reference an existing database, use `Database.Named(name)` as it is a
compile error to create duplicate databases.

#### Extends

- `BaseQueryExecutor`

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L276" target="_blank" rel="noopener">source</a>

`new SQLDatabase(name, cfg?): SQLDatabase`

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

##### acquire() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L301" target="_blank" rel="noopener">source</a>

`acquire(): Promise<Connection>`

Acquires a database connection from the database pool.

When the connection is closed or is garbage-collected, it is returned to the pool.

###### Returns

`Promise`\<[`Connection`](#connection)\>

a new connection to the database

##### begin() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L312" target="_blank" rel="noopener">source</a>

`begin(): Promise<Transaction>`

Begins a database transaction.

Make sure to always call `rollback` or `commit` to prevent hanging transactions.

###### Returns

`Promise`\<[`Transaction`](#transaction)\>

a transaction object that implements AsycDisposable

##### exec() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L229" target="_blank" rel="noopener">source</a>

`exec(strings, ...params): Promise<void>`

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

##### query() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L69" target="_blank" rel="noopener">source</a>

`query<T>(strings, ...params): AsyncGenerator<T>`

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

##### queryAll() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L129" target="_blank" rel="noopener">source</a>

`queryAll<T>(strings, ...params): Promise<T[]>`

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

##### queryRow() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L184" target="_blank" rel="noopener">source</a>

`queryRow<T>(strings, ...params): Promise<T | null>`

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

##### rawExec() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L255" target="_blank" rel="noopener">source</a>

`rawExec(query, ...params): Promise<void>`

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

##### rawQuery() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L101" target="_blank" rel="noopener">source</a>

`rawQuery<T>(query, ...params): AsyncGenerator<T>`

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

##### rawQueryAll() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L158" target="_blank" rel="noopener">source</a>

`rawQueryAll<T>(query, ...params): Promise<T[]>`

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

##### rawQueryRow() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L211" target="_blank" rel="noopener">source</a>

`rawQueryRow<T>(query, ...params): Promise<T | null>`

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

##### named() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L284" target="_blank" rel="noopener">source</a>

`static named<name>(name): SQLDatabase`

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
### Transaction <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L324" target="_blank" rel="noopener">source</a>

Represents a database transaction.

Make sure to always call `rollback` or `commit` to prevent hanging transactions.

#### Extends

- `BaseQueryExecutor`

#### Implements

- `AsyncDisposable`

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L328" target="_blank" rel="noopener">source</a>

`new Transaction(impl): Transaction`

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

##### \[asyncDispose\]() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L350" target="_blank" rel="noopener">source</a>

`asyncDispose: Promise<void>`

###### Returns

`Promise`\<`void`\>

###### Implementation of

`AsyncDisposable.[asyncDispose]`

##### commit() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L335" target="_blank" rel="noopener">source</a>

`commit(): Promise<void>`

Commit the transaction.

###### Returns

`Promise`\<`void`\>

##### exec() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L229" target="_blank" rel="noopener">source</a>

`exec(strings, ...params): Promise<void>`

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

##### query() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L69" target="_blank" rel="noopener">source</a>

`query<T>(strings, ...params): AsyncGenerator<T>`

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

##### queryAll() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L129" target="_blank" rel="noopener">source</a>

`queryAll<T>(strings, ...params): Promise<T[]>`

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

##### queryRow() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L184" target="_blank" rel="noopener">source</a>

`queryRow<T>(strings, ...params): Promise<T | null>`

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

##### rawExec() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L255" target="_blank" rel="noopener">source</a>

`rawExec(query, ...params): Promise<void>`

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

##### rawQuery() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L101" target="_blank" rel="noopener">source</a>

`rawQuery<T>(query, ...params): AsyncGenerator<T>`

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

##### rawQueryAll() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L158" target="_blank" rel="noopener">source</a>

`rawQueryAll<T>(query, ...params): Promise<T[]>`

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

##### rawQueryRow() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L211" target="_blank" rel="noopener">source</a>

`rawQueryRow<T>(query, ...params): Promise<T | null>`

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

##### rollback() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L344" target="_blank" rel="noopener">source</a>

`rollback(): Promise<void>`

Rollback the transaction.

###### Returns

`Promise`\<`void`\>

<!-- symbol-end -->

## Interfaces

<!-- symbol-start: SQLDatabaseConfig -->
### SQLDatabaseConfig <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L15" target="_blank" rel="noopener">source</a>

Configuration for a `SQLDatabase`.

#### Properties

##### migrations?

`optional migrations?: string | SQLMigrationsConfig`

***

<!-- symbol-end -->

<!-- symbol-start: SQLMigrationsConfig -->
### SQLMigrationsConfig <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L8" target="_blank" rel="noopener">source</a>

Configures how database migrations are managed for a `SQLDatabase`.

#### Properties

##### path

`path: string`

##### source?

`optional source?: "prisma" | "drizzle" | "drizzle/v1"`

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: Primitive -->
### Primitive <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L28" target="_blank" rel="noopener">source</a>

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

Represents a type that can be used in query template literals

***

<!-- symbol-end -->

<!-- symbol-start: ResultRow -->
### ResultRow <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/sqldb/database.ts#L25" target="_blank" rel="noopener">source</a>

`type ResultRow = Record<string, any>`

Represents a single row from a query result


<!-- symbol-end -->