import { getCurrentRequest } from "../../internal/reqtrack/mod";
import * as runtime from "../../internal/runtime/mod";
import { StringLiteral } from "../../internal/utils/constraints";

export interface SQLDatabaseConfig {
  migrations?: string;
}

const driverName = "node-pg";

/**
 * Represents a single row from a query result
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Row = Record<string, any>;

/** Represents a type that can be used in query template literals */
export type Primitive = string | number | boolean | null;

/**
 * Constructing a new database object will result in Encore provisioning a database with
 * that name and returning this object to represent it.
 *
 * If you want to reference an existing database, use `Database.Named(name)` as it is a
 * compile error to create duplicate databases.
 */
export class SQLDatabase {
  private readonly impl: runtime.SQLDatabase;

  /**
   * Creates a new database with the given name and configuration
   */
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  constructor(name: string, cfg?: SQLDatabaseConfig) {
    this.impl = runtime.RT.sqlDatabase(name);
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
    const args = new runtime.QueryArgs(params);
    const source = getCurrentRequest();
    const cursor = await this.impl.query(query, args, source);
    while (true) {
      const row = await cursor.next();
      if (row === null) {
        break;
      }
      yield row.values() as T;
    }
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
    const args = new runtime.QueryArgs(params);
    const source = getCurrentRequest();
    const result = await this.impl.query(query, args, source);
    while (true) {
      const row = await result.next();
      return row ? (row.values() as T) : null;
    }
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
    const args = new runtime.QueryArgs(params);
    const source = getCurrentRequest();

    // Need to await the cursor to process any errors from things like
    // unique constraint violations.
    let cur = await this.impl.query(query, args, source);
    await cur.next();
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
}

/**
 * Represents a dedicated connection to a database.
 */
export class Connection {
  private readonly impl: runtime.SQLConn;

  constructor(impl: runtime.SQLConn) {
    this.impl = impl;
  }

  /**
   * Returns the connection to the database pool.
   */
  async close() {
    await this.impl.close();
  }

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
    const args = new runtime.QueryArgs(params);
    const source = getCurrentRequest();
    const cursor = await this.impl.query(query, args, source);
    while (true) {
      const row = await cursor.next();
      if (row === null) {
        break;
      }
      yield row.values() as T;
    }
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
    const args = new runtime.QueryArgs(params);
    const source = getCurrentRequest();
    const result = await this.impl.query(query, args, source);
    while (true) {
      const row = await result.next();
      return row ? (row.values() as T) : null;
    }
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
    const args = new runtime.QueryArgs(params);
    const source = getCurrentRequest();

    // Need to await the cursor to process any errors from things like
    // unique constraint violations.
    let cur = await this.impl.query(query, args, source);
    await cur.next();
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
