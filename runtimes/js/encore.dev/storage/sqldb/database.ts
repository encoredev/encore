import { getCurrentRequest } from "../../internal/reqtrack/mod";
import * as runtime from "../../internal/runtime/mod";
import { StringLiteral } from "../../internal/utils/constraints";

export interface SQLMigrationsConfig {
  path: string;
  source?: "prisma" | "drizzle";
}
export interface SQLDatabaseConfig {
  migrations?: string | SQLMigrationsConfig;
}

const driverName = "node-pg";

/**
 * Represents a single row from a query result
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Row = Record<string, any>;

/** Represents a type that can be used in query template literals */
export type Primitive =
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
  | null
  | undefined;

type SQLQueryExecutor =
  | runtime.SQLConn
  | runtime.SQLDatabase
  | runtime.Transaction;

/** Base class containing shared query functionality */
class BaseQueryExecutor {
  constructor(protected readonly impl: SQLQueryExecutor) {}

  /**
   * query queries the database using a template string, replacing your placeholders in the template
   * with parametrised values without risking SQL injections.
   *
   * It returns an async generator, that allows iterating over the results
   * in a streaming fashion using `for await`.
   *
   * @example
   *
   * const email = "foo@example.com";
   * const result = database.query`SELECT id FROM users WHERE email=${email}`
   *
   * This produces the query: "SELECT id FROM users WHERE email=$1".
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async *query<T extends Row = Record<string, any>>(
    strings: TemplateStringsArray,
    ...params: Primitive[]
  ): AsyncGenerator<T> {
    const query = buildQuery(strings, params);
    const args = buildQueryArgs(params);
    const source = getCurrentRequest();
    const cursor = await this.impl.query(query, args, source);
    while (true) {
      const row = await cursor.next();
      if (row === null) break;
      yield row.values() as T;
    }
  }

  /**
   * rawQuery queries the database using a raw parametrised SQL query and parameters.
   *
   * It returns an async generator, that allows iterating over the results
   * in a streaming fashion using `for await`.
   *
   * @example
   * const query = "SELECT id FROM users WHERE email=$1";
   * const email = "foo@example.com";
   * for await (const row of database.rawQuery(query, email)) {
   *   console.log(row);
   * }
   *
   * @param query - The raw SQL query string.
   * @param params - The parameters to be used in the query.
   * @returns An async generator that yields rows from the query result.
   */
  async *rawQuery<T extends Row = Record<string, any>>(
    query: string,
    ...params: Primitive[]
  ): AsyncGenerator<T> {
    const args = buildQueryArgs(params);
    const source = getCurrentRequest();
    const result = await this.impl.query(query, args, source);
    while (true) {
      const row = await result.next();
      if (row === null) break;
      yield row.values() as T;
    }
  }

  /**
   * queryAll queries the database using a template string, replacing your placeholders in the template
   * with parametrised values without risking SQL injections.
   *
   * It returns an array of all results.
   *
   * @example
   *
   * const email = "foo@example.com";
   * const result = database.queryAll`SELECT id FROM users WHERE email=${email}`
   *
   * This produces the query: "SELECT id FROM users WHERE email=$1".
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async queryAll<T extends Row = Record<string, any>>(
    strings: TemplateStringsArray,
    ...params: Primitive[]
  ): Promise<T[]> {
    const query = buildQuery(strings, params);
    const args = buildQueryArgs(params);
    const source = getCurrentRequest();
    const cursor = await this.impl.query(query, args, source);
    const result: T[] = [];
    while (true) {
      const row = await cursor.next();
      if (row === null) break;
      result.push(row.values() as T);
    }
    return result;
  }

  /**
   * rawQueryAll queries the database using a raw parametrised SQL query and parameters.
   *
   * It returns an array of all results.
   *
   * @example
   *
   * const query = "SELECT id FROM users WHERE email=$1";
   * const email = "foo@example.com";
   * const rows = await database.rawQueryAll(query, email);
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async rawQueryAll<T extends Row = Record<string, any>>(
    query: string,
    ...params: Primitive[]
  ): Promise<T[]> {
    const args = buildQueryArgs(params);
    const source = getCurrentRequest();
    const cursor = await this.impl.query(query, args, source);
    const result: T[] = [];
    while (true) {
      const row = await cursor.next();
      if (row === null) break;
      result.push(row.values() as T);
    }
    return result;
  }

  /**
   * queryRow is like query but returns only a single row.
   * If the query selects no rows it returns null.
   * Otherwise it returns the first row and discards the rest.
   *
   * @example
   * const email = "foo@example.com";
   * const result = database.queryRow`SELECT id FROM users WHERE email=${email}`
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async queryRow<T extends Row = Record<string, any>>(
    strings: TemplateStringsArray,
    ...params: Primitive[]
  ): Promise<T | null> {
    const query = buildQuery(strings, params);
    const args = buildQueryArgs(params);
    const source = getCurrentRequest();
    const result = await this.impl.query(query, args, source);
    const row = await result.next();
    return row ? (row.values() as T) : null;
  }

  /**
   * rawQueryRow is like rawQuery but returns only a single row.
   * If the query selects no rows, it returns null.
   * Otherwise, it returns the first row and discards the rest.
   *
   * @example
   * const query = "SELECT id FROM users WHERE email=$1";
   * const email = "foo@example.com";
   * const result = await database.rawQueryRow(query, email);
   * console.log(result);
   *
   * @param query - The raw SQL query string.
   * @param params - The parameters to be used in the query.
   * @returns A promise that resolves to a single row or null.
   */
  async rawQueryRow<T extends Row = Record<string, any>>(
    query: string,
    ...params: Primitive[]
  ): Promise<T | null> {
    const args = buildQueryArgs(params);
    const source = getCurrentRequest();
    const result = await this.impl.query(query, args, source);
    const row = await result.next();
    return row ? (row.values() as T) : null;
  }

  /**
   * exec executes a query without returning any rows.
   *
   * @example
   * const email = "foo@example.com";
   * const result = database.exec`DELETE FROM users WHERE email=${email}`
   */
  async exec(
    strings: TemplateStringsArray,
    ...params: Primitive[]
  ): Promise<void> {
    const query = buildQuery(strings, params);
    const args = buildQueryArgs(params);
    const source = getCurrentRequest();

    // Need to await the cursor to process any errors from things like
    // unique constraint violations.
    const cur = await this.impl.query(query, args, source);
    await cur.next();
  }

  /**
   * rawExec executes a query without returning any rows.
   *
   * @example
   * const query = "DELETE FROM users WHERE email=$1";
   * const email = "foo@example.com";
   * await database.rawExec(query, email);
   *
   * @param query - The raw SQL query string.
   * @param params - The parameters to be used in the query.
   * @returns A promise that resolves when the query has been executed.
   */
  async rawExec(query: string, ...params: Primitive[]): Promise<void> {
    const args = buildQueryArgs(params);
    const source = getCurrentRequest();

    // Need to await the cursor to process any errors from things like
    // unique constraint violations.
    const cur = await this.impl.query(query, args, source);
    await cur.next();
  }
}

/**
 * Constructing a new database object will result in Encore provisioning a database with
 * that name and returning this object to represent it.
 *
 * If you want to reference an existing database, use `Database.Named(name)` as it is a
 * compile error to create duplicate databases.
 */
export class SQLDatabase extends BaseQueryExecutor {
  protected declare readonly impl: runtime.SQLDatabase;

  constructor(name: string, cfg?: SQLDatabaseConfig) {
    super(runtime.RT.sqlDatabase(name));
  }

  /**
   * Reference an existing database by name, if the database doesn't
   * exist yet, use `new Database(name)` instead.
   */
  static named<name extends string>(name: StringLiteral<name>): SQLDatabase {
    return new SQLDatabase(name);
  }

  /**
   * Returns the connection string for the database
   */
  get connectionString(): string {
    return this.impl.connString();
  }

  /**
   * Acquires a database connection from the database pool.
   *
   * When the connection is closed or is garbage-collected, it is returned to the pool.
   * @returns a new connection to the database
   */
  async acquire(): Promise<Connection> {
    const impl = await this.impl.acquire();
    return new Connection(impl);
  }

  /**
   * Begins a database transaction.
   *
   * Make sure to always call `rollback` or `commit` to prevent hanging transactions.
   * @returns a transaction object that implements AsycDisposable
   */
  async begin(): Promise<Transaction> {
    const source = getCurrentRequest();
    const impl = await this.impl.begin(source);
    return new Transaction(impl);
  }
}

export class Transaction extends BaseQueryExecutor implements AsyncDisposable {
  protected declare readonly impl: runtime.Transaction;
  private done: boolean = false;

  constructor(impl: runtime.Transaction) {
    super(impl);
  }

  /**
   * Commit the transaction.
   */
  async commit() {
    this.done = true;
    const source = getCurrentRequest();
    await this.impl.commit(source);
  }

  /**
   * Rollback the transaction.
   */
  async rollback() {
    this.done = true;
    const source = getCurrentRequest();
    await this.impl.rollback(source);
  }

  async [Symbol.asyncDispose]() {
    if (!this.done) {
      await this.rollback();
    }
  }
}

/**
 * Represents a dedicated connection to a database.
 */
export class Connection extends BaseQueryExecutor {
  protected declare readonly impl: runtime.SQLConn;

  constructor(impl: runtime.SQLConn) {
    super(impl);
  }

  /**
   * Returns the connection to the database pool.
   */
  async close() {
    await this.impl.close();
  }
}

function buildQuery(strings: TemplateStringsArray, expr: Primitive[]): string {
  let query = "";
  for (let i = 0; i < strings.length; i++) {
    query += strings[i];

    if (i < expr.length) {
      query += "$" + (i + 1);
    }
  }

  // return queryWithComment(query, driverName);
  return query;
}

function buildQueryArgs(params: Primitive[]): runtime.QueryArgs {
  // Convert undefined to null.
  return new runtime.QueryArgs(params.map((p) => p ?? null));
}
