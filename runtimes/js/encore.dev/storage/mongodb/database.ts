import { getCurrentRequest } from "../../internal/reqtrack/mod";
import * as runtime from "../../internal/runtime/mod";
import { StringLiteral } from "../../internal/utils/constraints";

/**
 * Configuration options for declaring a MongoDB database.
 * There are no options yet.
 */
// eslint-disable-next-line @typescript-eslint/no-empty-interface
export interface MongoDatabaseConfig {}

/** A MongoDB document. */
export type Document = Record<string, any>;

/** Options for findOne. */
export interface FindOneOptions {
  /** The order to sort matching documents in, e.g. `{ createdAt: -1 }`. */
  sort?: Document;
  /** The fields to include or exclude, e.g. `{ url: 1 }`. */
  projection?: Document;
  /** The number of matching documents to skip. */
  skip?: number;
}

/** Options for find. */
export interface FindOptions extends FindOneOptions {
  /** The maximum number of documents to return. */
  limit?: number;
}

/** Options for updateOne, updateMany and replaceOne. */
export interface UpdateOptions {
  /** Insert a new document if no document matches the filter. */
  upsert?: boolean;
}

/** Options for createIndex. */
export interface CreateIndexOptions {
  /** The name of the index. MongoDB generates one if unset. */
  name?: string;
  /** Reject documents whose indexed fields duplicate another document's. */
  unique?: boolean;
  /** Only index documents that have the indexed fields. */
  sparse?: boolean;
  /** Delete documents this many seconds after the date in the indexed field. */
  expireAfterSeconds?: number;
}

export interface InsertOneResult {
  /** The `_id` of the inserted document. ObjectIds are returned as hex strings. */
  insertedId: any;
}

export interface InsertManyResult {
  /** The `_id`s of the inserted documents, in order. */
  insertedIds: any[];
}

export interface UpdateResult {
  /** The number of documents that matched the filter. */
  matchedCount: number;
  /** The number of documents that were modified. */
  modifiedCount: number;
  /** The `_id` of the inserted document, if the update was an upsert that inserted one. */
  upsertedId?: any;
}

export interface DeleteResult {
  /** The number of documents that were deleted. */
  deletedCount: number;
}

/** An index on a collection. */
export interface IndexInfo {
  /** The indexed fields, e.g. `{ email: 1 }`. */
  key: Document;
  /** The name of the index. */
  name?: string;
}

/**
 * MongoError is an error reported by MongoDB.
 */
export class MongoError extends Error {
  /**
   * The MongoDB error code, e.g. 11000 for a duplicate key,
   * or undefined if the error did not come from the server.
   */
  readonly code: number | undefined;

  /**
   * The MongoDB error labels, e.g. "TransientTransactionError",
   * which tell whether the operation or transaction can be retried.
   */
  readonly errorLabels: string[];

  constructor(message: string, code?: number, errorLabels: string[] = []) {
    super(message);
    this.name = "MongoError";
    this.code = code;
    this.errorLabels = errorLabels;
    Object.setPrototypeOf(this, MongoError.prototype);
  }

  /** Reports whether the error has the given label. */
  hasErrorLabel(label: string): boolean {
    return this.errorLabels.includes(label);
  }
}

// Matches the prefix the runtime adds to errors from MongoDB:
// "mongodb error [code=<code>] [labels=<label>,<label>]: ".
const errorPrefix = /^mongodb error(?: code=(-?\d+))?(?: labels=([^:\s]+))?: /;

/** Converts an error from the runtime into a MongoError. */
function toMongoError(err: unknown): unknown {
  if (!(err instanceof Error)) {
    return err;
  }
  const m = errorPrefix.exec(err.message);
  if (m) {
    const code = m[1] !== undefined ? Number(m[1]) : undefined;
    const labels = m[2] !== undefined ? m[2].split(",") : [];
    return new MongoError(err.message.slice(m[0].length), code, labels);
  }
  return new MongoError(err.message);
}

const transientTransactionError = "TransientTransactionError";
const unknownTransactionCommitResult = "UnknownTransactionCommitResult";
const maxTimeMSExpired = 50;

/** How long withTransaction keeps retrying, like the MongoDB drivers. */
const maxTransactionRetryTimeMs = 120_000;

function hasLabel(err: unknown, label: string): boolean {
  return err instanceof MongoError && err.hasErrorLabel(label);
}

/** The delay before retry number attempt, like the MongoDB drivers. */
function backoffMs(attempt: number): number {
  const jitter = Math.random();
  return Math.min(jitter * 5 * Math.pow(1.5, attempt), jitter * 500);
}

async function call<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn();
  } catch (err) {
    throw toMongoError(err);
  }
}

/**
 * MongoDatabase represents a MongoDB database, provisioned and
 * connected to by Encore.
 *
 * @example
 * ```ts
 * import { MongoDatabase } from "encore.dev/storage/mongodb";
 *
 * export const db = new MongoDatabase("urls");
 *
 * await db.collection("urls").insertOne({ _id: id, url });
 * const u = await db.collection("urls").findOne({ _id: id });
 * ```
 */
export class MongoDatabase {
  protected readonly impl: runtime.MongoDatabase;

  /**
   * Declares a new MongoDB database.
   *
   * The name must be unique within the application, and defined in
   * snake_case. Encore uses static analysis to find databases, so it
   * must be a string literal.
   */
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  constructor(name: string, cfg?: MongoDatabaseConfig) {
    this.impl = runtime.RT.mongoDatabase(name);
  }

  /**
   * Reference an existing MongoDB database by name, declared elsewhere
   * with `new MongoDatabase(...)`. Use it to access another service's
   * database without importing that service's code.
   */
  static named<name extends string>(name: StringLiteral<name>): MongoDatabase {
    return new MongoDatabase(name);
  }

  /**
   * Returns the collection with the given name.
   * MongoDB creates the collection when the first document is inserted.
   */
  collection<T extends Document = Document>(name: string): Collection<T> {
    return new Collection<T>(this.impl.collection(name));
  }

  /**
   * Starts a transaction. Use the transaction's collections for the
   * operations that belong to it, then commit or abort it.
   *
   * `Transaction` implements `AsyncDisposable`, so with `await using`
   * it is aborted automatically if it is neither committed nor aborted:
   *
   * ```ts
   * await using tx = await db.begin();
   * await tx.collection("accounts").updateOne(...);
   * await tx.commit();
   * ```
   *
   * Prefer {@link withTransaction}, which commits, aborts and retries for you.
   */
  async begin(): Promise<Transaction> {
    return new Transaction(await call(() => this.impl.begin()));
  }

  /**
   * Runs fn in a transaction, and commits it if fn returns.
   * If fn throws, the transaction is aborted and the error is rethrown.
   *
   * Like the MongoDB drivers' withTransaction, it retries when MongoDB
   * reports the transaction can be retried, for up to 120 seconds:
   * the whole transaction (calling fn again) on a
   * "TransientTransactionError", and only the commit on an
   * "UnknownTransactionCommitResult". So fn may run more than once,
   * and must only change the database through the transaction.
   *
   * Use the collections of the transaction passed to fn:
   *
   * ```ts
   * await db.withTransaction(async (tx) => {
   *   await tx.collection("accounts").updateOne({ _id: from }, { $inc: { balance: -10 } });
   *   await tx.collection("accounts").updateOne({ _id: to }, { $inc: { balance: 10 } });
   * });
   * ```
   */
  async withTransaction<R>(fn: (tx: Transaction) => Promise<R>): Promise<R> {
    const start = Date.now();
    let lastError: unknown;

    for (let attempt = 0; ; attempt++) {
      if (attempt > 0) {
        const delay = backoffMs(attempt);
        if (Date.now() - start + delay >= maxTransactionRetryTimeMs) {
          throw lastError;
        }
        await new Promise((resolve) => setTimeout(resolve, delay));
      }

      const tx = await this.begin();
      let result: R;
      try {
        result = await fn(tx);
      } catch (err) {
        await tx.abort().catch(() => {}); // the original error matters, not the abort's
        if (hasLabel(err, transientTransactionError)) {
          lastError = err;
          continue; // retry the whole transaction
        }
        throw err;
      }

      for (;;) {
        try {
          await tx.commit();
          return result;
        } catch (err) {
          if (
            hasLabel(err, unknownTransactionCommitResult) &&
            (err as MongoError).code !== maxTimeMSExpired &&
            Date.now() - start < maxTransactionRetryTimeMs
          ) {
            continue; // retry only the commit
          }
          if (hasLabel(err, transientTransactionError)) {
            lastError = err;
            break; // retry the whole transaction
          }
          throw err;
        }
      }
    }
  }
}

/**
 * Transaction is a MongoDB transaction. Operations on its collections
 * run in the transaction until it is committed or aborted.
 */
export class Transaction implements AsyncDisposable {
  private done = false;

  constructor(private readonly impl: runtime.MongoTransaction) {}

  /** Returns the collection with the given name, in this transaction. */
  collection<T extends Document = Document>(name: string): Collection<T> {
    return new Collection<T>(this.impl.collection(name));
  }

  /** Commits the transaction. */
  async commit(): Promise<void> {
    this.done = true;
    const source = getCurrentRequest();
    return call(() => this.impl.commit(source));
  }

  /** Aborts the transaction, discarding its changes. */
  async abort(): Promise<void> {
    this.done = true;
    const source = getCurrentRequest();
    return call(() => this.impl.abort(source));
  }

  async [Symbol.asyncDispose]() {
    if (!this.done) {
      await this.abort();
    }
  }
}

/**
 * Collection is a MongoDB collection.
 *
 * Values are converted between JavaScript and MongoDB as follows:
 * objects, arrays, strings, numbers, booleans, null and Date are stored
 * as their MongoDB equivalents. Values with no JavaScript equivalent
 * are read as strings: an ObjectId as its 24 character hex form.
 *
 * Operations that fail throw a {@link MongoError}.
 */
export class Collection<T extends Document = Document> {
  constructor(private readonly impl: runtime.MongoCollection) {}

  /** Inserts a document. */
  async insertOne(doc: T): Promise<InsertOneResult> {
    const source = getCurrentRequest();
    const insertedId = await call(() => this.impl.insertOne(doc, source));
    return { insertedId };
  }

  /** Inserts documents. */
  async insertMany(docs: T[]): Promise<InsertManyResult> {
    const source = getCurrentRequest();
    const insertedIds = await call(() => this.impl.insertMany(docs, source));
    return { insertedIds };
  }

  /** Returns the first document matching the filter, or null if there is none. */
  async findOne(filter: Document, options?: FindOneOptions): Promise<T | null> {
    const source = getCurrentRequest();
    const doc = await call(() =>
      this.impl.findOne(
        filter,
        options?.sort,
        options?.projection,
        options?.skip,
        source
      )
    );
    return doc as T | null;
  }

  /** Returns all documents matching the filter. */
  async find(filter: Document, options?: FindOptions): Promise<T[]> {
    const source = getCurrentRequest();
    const docs = await call(() =>
      this.impl.find(
        filter,
        options?.sort,
        options?.projection,
        options?.skip,
        options?.limit,
        source
      )
    );
    return docs as T[];
  }

  /** Updates the first document matching the filter, e.g. with `{ $set: { url } }`. */
  async updateOne(
    filter: Document,
    update: Document,
    options?: UpdateOptions
  ): Promise<UpdateResult> {
    const source = getCurrentRequest();
    return call(() => this.impl.updateOne(filter, update, options?.upsert, source));
  }

  /** Updates all documents matching the filter. */
  async updateMany(
    filter: Document,
    update: Document,
    options?: UpdateOptions
  ): Promise<UpdateResult> {
    const source = getCurrentRequest();
    return call(() => this.impl.updateMany(filter, update, options?.upsert, source));
  }

  /** Replaces the first document matching the filter. */
  async replaceOne(
    filter: Document,
    replacement: T,
    options?: UpdateOptions
  ): Promise<UpdateResult> {
    const source = getCurrentRequest();
    return call(() =>
      this.impl.replaceOne(filter, replacement, options?.upsert, source)
    );
  }

  /** Deletes the first document matching the filter. */
  async deleteOne(filter: Document): Promise<DeleteResult> {
    const source = getCurrentRequest();
    const deletedCount = await call(() => this.impl.deleteOne(filter, source));
    return { deletedCount };
  }

  /** Deletes all documents matching the filter. */
  async deleteMany(filter: Document): Promise<DeleteResult> {
    const source = getCurrentRequest();
    const deletedCount = await call(() => this.impl.deleteMany(filter, source));
    return { deletedCount };
  }

  /** Counts the documents matching the filter. */
  async countDocuments(filter: Document = {}): Promise<number> {
    const source = getCurrentRequest();
    return call(() => this.impl.countDocuments(filter, source));
  }

  /** Runs an aggregation pipeline and returns the resulting documents. */
  async aggregate<R extends Document = Document>(pipeline: Document[]): Promise<R[]> {
    const source = getCurrentRequest();
    const docs = await call(() => this.impl.aggregate(pipeline, source));
    return docs as R[];
  }

  /**
   * Creates an index on the given fields and returns its name,
   * e.g. `createIndex({ email: 1 }, { unique: true })`.
   * Creating an index that already exists does nothing.
   */
  async createIndex(keys: Document, options?: CreateIndexOptions): Promise<string> {
    const source = getCurrentRequest();
    return call(() => this.impl.createIndex(keys, options, source));
  }

  /** Lists the indexes on the collection. */
  async listIndexes(): Promise<IndexInfo[]> {
    const source = getCurrentRequest();
    const indexes = await call(() => this.impl.listIndexes(source));
    return indexes as IndexInfo[];
  }

  /** Drops the index with the given name. */
  async dropIndex(name: string): Promise<void> {
    const source = getCurrentRequest();
    return call(() => this.impl.dropIndex(name, source));
  }
}

/**
 * Reports whether err is a duplicate key error, returned when a
 * document violates a unique index (including on `_id`).
 */
export function isDuplicateKeyError(err: unknown): boolean {
  return err instanceof MongoError && err.code === 11000;
}
