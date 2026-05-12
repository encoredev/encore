---
title: encore.dev/storage/sqldb
lang: ts
toc: true
---

# encore.dev/storage/sqldb

## Classes

### Connection

Defined in: [storage/sqldb/database.ts:360](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L360)

Represents a dedicated connection to a database.

#### Extends

- `BaseQueryExecutor`

#### Constructors

##### Constructor

```ts
new Connection(impl): Connection;
```

Defined in: [storage/sqldb/database.ts:363](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L363)

###### Parameters

###### impl

`SQLConn`

###### Returns

[`Connection`](#connection)

###### Overrides

```ts
BaseQueryExecutor.constructor
```

#### Properties

##### impl

```ts
protected readonly impl: SQLConn;
```

Defined in: [storage/sqldb/database.ts:361](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L361)

###### Overrides

```ts
BaseQueryExecutor.impl
```

#### Methods

##### close()

```ts
close(): Promise<void>;
```

Defined in: [storage/sqldb/database.ts:370](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L370)

Returns the connection to the database pool.

###### Returns

`Promise`\<`void`\>

##### exec()

```ts
exec(strings, ...params): Promise<void>;
```

Defined in: [storage/sqldb/database.ts:229](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L229)

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

```ts
BaseQueryExecutor.exec
```

##### query()

```ts
query<T>(strings, ...params): AsyncGenerator<T>;
```

Defined in: [storage/sqldb/database.ts:69](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L69)

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

```ts
BaseQueryExecutor.query
```

##### queryAll()

```ts
queryAll<T>(strings, ...params): Promise<T[]>;
```

Defined in: [storage/sqldb/database.ts:129](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L129)

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

```ts
BaseQueryExecutor.queryAll
```

##### queryRow()

```ts
queryRow<T>(strings, ...params): Promise<T | null>;
```

Defined in: [storage/sqldb/database.ts:184](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L184)

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

```ts
BaseQueryExecutor.queryRow
```

##### rawExec()

```ts
rawExec(query, ...params): Promise<void>;
```

Defined in: [storage/sqldb/database.ts:255](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L255)

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

```ts
BaseQueryExecutor.rawExec
```

##### rawQuery()

```ts
rawQuery<T>(query, ...params): AsyncGenerator<T>;
```

Defined in: [storage/sqldb/database.ts:101](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L101)

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

```ts
BaseQueryExecutor.rawQuery
```

##### rawQueryAll()

```ts
rawQueryAll<T>(query, ...params): Promise<T[]>;
```

Defined in: [storage/sqldb/database.ts:158](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L158)

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

```ts
BaseQueryExecutor.rawQueryAll
```

##### rawQueryRow()

```ts
rawQueryRow<T>(query, ...params): Promise<T | null>;
```

Defined in: [storage/sqldb/database.ts:211](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L211)

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

```ts
BaseQueryExecutor.rawQueryRow
```

***

### SQLDatabase

Defined in: [storage/sqldb/database.ts:273](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L273)

Constructing a new database object will result in Encore provisioning a database with
that name and returning this object to represent it.

If you want to reference an existing database, use `Database.Named(name)` as it is a
compile error to create duplicate databases.

#### Extends

- `BaseQueryExecutor`

#### Constructors

##### Constructor

```ts
new SQLDatabase(name, cfg?): SQLDatabase;
```

Defined in: [storage/sqldb/database.ts:276](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L276)

###### Parameters

###### name

`string`

###### cfg?

[`SQLDatabaseConfig`](#sqldatabaseconfig)

###### Returns

[`SQLDatabase`](#sqldatabase)

###### Overrides

```ts
BaseQueryExecutor.constructor
```

#### Properties

##### impl

```ts
protected readonly impl: SQLDatabase;
```

Defined in: [storage/sqldb/database.ts:274](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L274)

###### Overrides

```ts
BaseQueryExecutor.impl
```

#### Accessors

##### connectionString

###### Get Signature

```ts
get connectionString(): string;
```

Defined in: [storage/sqldb/database.ts:291](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L291)

Returns the connection string for the database

###### Returns

`string`

#### Methods

##### acquire()

```ts
acquire(): Promise<Connection>;
```

Defined in: [storage/sqldb/database.ts:301](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L301)

Acquires a database connection from the database pool.

When the connection is closed or is garbage-collected, it is returned to the pool.

###### Returns

`Promise`\<[`Connection`](#connection)\>

a new connection to the database

##### begin()

```ts
begin(): Promise<Transaction>;
```

Defined in: [storage/sqldb/database.ts:312](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L312)

Begins a database transaction.

Make sure to always call `rollback` or `commit` to prevent hanging transactions.

###### Returns

`Promise`\<[`Transaction`](#transaction)\>

a transaction object that implements AsycDisposable

##### exec()

```ts
exec(strings, ...params): Promise<void>;
```

Defined in: [storage/sqldb/database.ts:229](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L229)

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

```ts
BaseQueryExecutor.exec
```

##### query()

```ts
query<T>(strings, ...params): AsyncGenerator<T>;
```

Defined in: [storage/sqldb/database.ts:69](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L69)

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

```ts
BaseQueryExecutor.query
```

##### queryAll()

```ts
queryAll<T>(strings, ...params): Promise<T[]>;
```

Defined in: [storage/sqldb/database.ts:129](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L129)

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

```ts
BaseQueryExecutor.queryAll
```

##### queryRow()

```ts
queryRow<T>(strings, ...params): Promise<T | null>;
```

Defined in: [storage/sqldb/database.ts:184](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L184)

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

```ts
BaseQueryExecutor.queryRow
```

##### rawExec()

```ts
rawExec(query, ...params): Promise<void>;
```

Defined in: [storage/sqldb/database.ts:255](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L255)

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

```ts
BaseQueryExecutor.rawExec
```

##### rawQuery()

```ts
rawQuery<T>(query, ...params): AsyncGenerator<T>;
```

Defined in: [storage/sqldb/database.ts:101](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L101)

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

```ts
BaseQueryExecutor.rawQuery
```

##### rawQueryAll()

```ts
rawQueryAll<T>(query, ...params): Promise<T[]>;
```

Defined in: [storage/sqldb/database.ts:158](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L158)

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

```ts
BaseQueryExecutor.rawQueryAll
```

##### rawQueryRow()

```ts
rawQueryRow<T>(query, ...params): Promise<T | null>;
```

Defined in: [storage/sqldb/database.ts:211](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L211)

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

```ts
BaseQueryExecutor.rawQueryRow
```

##### named()

```ts
static named<name>(name): SQLDatabase;
```

Defined in: [storage/sqldb/database.ts:284](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L284)

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

### Transaction

Defined in: [storage/sqldb/database.ts:324](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L324)

Represents a database transaction.

Make sure to always call `rollback` or `commit` to prevent hanging transactions.

#### Extends

- `BaseQueryExecutor`

#### Implements

- `AsyncDisposable`

#### Constructors

##### Constructor

```ts
new Transaction(impl): Transaction;
```

Defined in: [storage/sqldb/database.ts:328](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L328)

###### Parameters

###### impl

`Transaction`

###### Returns

[`Transaction`](#transaction)

###### Overrides

```ts
BaseQueryExecutor.constructor
```

#### Properties

##### impl

```ts
protected readonly impl: Transaction;
```

Defined in: [storage/sqldb/database.ts:325](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L325)

###### Overrides

```ts
BaseQueryExecutor.impl
```

#### Methods

##### \[asyncDispose\]()

```ts
asyncDispose: Promise<void>;
```

Defined in: [storage/sqldb/database.ts:350](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L350)

###### Returns

`Promise`\<`void`\>

###### Implementation of

```ts
AsyncDisposable.[asyncDispose]
```

##### commit()

```ts
commit(): Promise<void>;
```

Defined in: [storage/sqldb/database.ts:335](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L335)

Commit the transaction.

###### Returns

`Promise`\<`void`\>

##### exec()

```ts
exec(strings, ...params): Promise<void>;
```

Defined in: [storage/sqldb/database.ts:229](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L229)

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

```ts
BaseQueryExecutor.exec
```

##### query()

```ts
query<T>(strings, ...params): AsyncGenerator<T>;
```

Defined in: [storage/sqldb/database.ts:69](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L69)

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

```ts
BaseQueryExecutor.query
```

##### queryAll()

```ts
queryAll<T>(strings, ...params): Promise<T[]>;
```

Defined in: [storage/sqldb/database.ts:129](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L129)

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

```ts
BaseQueryExecutor.queryAll
```

##### queryRow()

```ts
queryRow<T>(strings, ...params): Promise<T | null>;
```

Defined in: [storage/sqldb/database.ts:184](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L184)

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

```ts
BaseQueryExecutor.queryRow
```

##### rawExec()

```ts
rawExec(query, ...params): Promise<void>;
```

Defined in: [storage/sqldb/database.ts:255](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L255)

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

```ts
BaseQueryExecutor.rawExec
```

##### rawQuery()

```ts
rawQuery<T>(query, ...params): AsyncGenerator<T>;
```

Defined in: [storage/sqldb/database.ts:101](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L101)

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

```ts
BaseQueryExecutor.rawQuery
```

##### rawQueryAll()

```ts
rawQueryAll<T>(query, ...params): Promise<T[]>;
```

Defined in: [storage/sqldb/database.ts:158](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L158)

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

```ts
BaseQueryExecutor.rawQueryAll
```

##### rawQueryRow()

```ts
rawQueryRow<T>(query, ...params): Promise<T | null>;
```

Defined in: [storage/sqldb/database.ts:211](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L211)

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

```ts
BaseQueryExecutor.rawQueryRow
```

##### rollback()

```ts
rollback(): Promise<void>;
```

Defined in: [storage/sqldb/database.ts:344](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L344)

Rollback the transaction.

###### Returns

`Promise`\<`void`\>

## Interfaces

### SQLDatabaseConfig

Defined in: [storage/sqldb/database.ts:15](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L15)

Configuration for a `SQLDatabase`.

#### Properties

##### migrations?

```ts
optional migrations?: string | SQLMigrationsConfig;
```

Defined in: [storage/sqldb/database.ts:16](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L16)

***

### SQLMigrationsConfig

Defined in: [storage/sqldb/database.ts:8](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L8)

Configures how database migrations are managed for a `SQLDatabase`.

#### Properties

##### path

```ts
path: string;
```

Defined in: [storage/sqldb/database.ts:9](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L9)

##### source?

```ts
optional source?: "prisma" | "drizzle" | "drizzle/v1";
```

Defined in: [storage/sqldb/database.ts:10](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L10)

## Type Aliases

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

Defined in: [storage/sqldb/database.ts:28](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L28)

Represents a type that can be used in query template literals

***

### ResultRow

```ts
type ResultRow = Record<string, any>;
```

Defined in: [storage/sqldb/database.ts:25](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/sqldb/database.ts#L25)

Represents a single row from a query result
