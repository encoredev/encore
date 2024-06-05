import { getCurrentRequest } from "../internal/reqtrack/mod";
import * as runtime from "../internal/runtime/mod";

export type { Logger };

/* eslint-disable @typescript-eslint/unified-signatures */
/**
 * A field value we support logging
 */
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-expect-error
export type FieldValue =
  | string
  | number
  | boolean
  | null
  | undefined
  | FieldsObject
  | FieldValue[];

/**
 * A map of fields that can be logged
 */
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-expect-error
export type FieldsObject = Record<string, FieldValue>;

export enum LogLevel {
  Trace = runtime.LogLevel.Trace,
  Debug = runtime.LogLevel.Debug,
  Info = runtime.LogLevel.Info,
  Warn = runtime.LogLevel.Warn,
  Error = runtime.LogLevel.Error,
}

class Logger {
  private impl: runtime.Logger;

  constructor(impl: runtime.Logger) {
    this.impl = impl;
  }

  /**
   * Returns a new logger with the specified level.
   */
  withLevel(level: LogLevel): Logger {
    return new Logger(this.impl.withLevel(level));
  }

  /**
   * Returns a new logger with the given fields added to the context.
   */
  with(fields: FieldsObject): Logger {
    return new Logger(this.impl.with(fields));
  }

  /**
   * Trace logs a message at the trace level.
   */
  trace(msg: string, fields?: FieldsObject) {
    this.log(runtime.LogLevel.Trace, msg, fields);
  }

  /**
   * Debug logs a message at the debug level.
   */
  debug(msg: string, fields?: FieldsObject) {
    this.log(runtime.LogLevel.Debug, msg, fields);
  }

  /**
   * Info logs a message at the info level.
   */
  info(msg: string, fields?: FieldsObject) {
    this.log(runtime.LogLevel.Info, msg, fields);
  }

  /**
   * Warn logs a message at the warn level.
   */
  warn(err: Error | unknown, fields?: FieldsObject): void;
  warn(err: Error | unknown, msg: string, fields?: FieldsObject): void;
  warn(msg: string, fields?: FieldsObject): void;
  warn(errOrMsg: unknown, msgOrFields: unknown, fields?: unknown) {
    this.log(runtime.LogLevel.Warn, errOrMsg, msgOrFields, fields);
  }

  error(err: Error | unknown, fields?: FieldsObject): void;
  error(err: Error | unknown, msg: string, fields?: FieldsObject): void;
  error(msg: string, fields?: FieldsObject): void;
  error(errOrMsg: unknown, msgOrFields: unknown, fields?: unknown) {
    this.log(runtime.LogLevel.Error, errOrMsg, msgOrFields, fields);
  }

  /**
   * The actual logging implementation.
   */
  private log(
    level: runtime.LogLevel,
    errOrMsg: unknown,
    msgOrFields?: unknown,
    possibleFields?: unknown
  ) {
    let err: Error | undefined;
    let msg: string;
    let fields: FieldsObject | undefined;

    // Parse the arguments
    if (typeof errOrMsg === "string") {
      // log(msg, fields?)
      err = undefined;
      msg = errOrMsg;
      fields = msgOrFields as FieldsObject | undefined;
    } else if (typeof msgOrFields === "string") {
      // log(err, msg, fields?)
      if (errOrMsg) {
        if (errOrMsg instanceof Error) {
          err = errOrMsg;
        } else {
          err = new Error(String(errOrMsg));
        }
      }
      msg = msgOrFields;
      fields = possibleFields as FieldsObject | undefined;
    } else {
      // log(err, fields?)
      if (errOrMsg) {
        if (errOrMsg instanceof Error) {
          err = errOrMsg;
        } else {
          err = new Error(String(errOrMsg));
        }
      }

      msg = "";
      fields = msgOrFields as FieldsObject | undefined;

      // if (possibleFields) {
      //   throw new Error("Invalid arguments to log");
      // }
    }

    const req = getCurrentRequest();

    this.impl.log(req, level, msg, err, undefined, fields);
  }
}

const log = new Logger(runtime.RT.logger());

/**
 * The default logger for the app
 */
export default log;

/**
 * Trace logs a message at the trace level
 */
export function trace(msg: string, fields?: FieldsObject) {
  log.trace(msg, fields);
}

/**
 * Debug logs a message at the debug level
 */
export function debug(msg: string, fields?: FieldsObject) {
  log.debug(msg, fields);
}

/**
 * Info logs a message at the info level
 */
export function info(msg: string, fields?: FieldsObject) {
  log.info(msg, fields);
}

/**
 * Warn logs a message at the warn level
 */
export function warn(err: Error | unknown, fields?: FieldsObject): void;
export function warn(
  err: Error | unknown,
  msg: string,
  fields?: FieldsObject
): void;
export function warn(msg: string, fields?: FieldsObject): void;
export function warn(
  errOrMsg: unknown,
  msgOrFields: unknown,
  fields?: unknown
) {
  // the type cast here is just for TSC to be happy - the underlying method uses the same overloads so
  // will type check the arguments correctly
  log.warn(
    errOrMsg as Error,
    msgOrFields as string,
    fields as FieldsObject | undefined
  );
}

/**
 * Error logs a message at the error level
 */
export function error(err: Error | unknown, fields?: FieldsObject): void;
export function error(
  err: Error | unknown,
  msg: string,
  fields?: FieldsObject
): void;
export function error(msg: string, fields?: FieldsObject): void;
export function error(
  errOrMsg: unknown,
  msgOrFields: unknown,
  fields?: unknown
) {
  // the type cast here is just for TSC to be happy - the underlying method uses the same overloads so
  // will type check the arguments correctly
  log.error(
    errOrMsg as Error,
    msgOrFields as string,
    fields as FieldsObject | undefined
  );
}
