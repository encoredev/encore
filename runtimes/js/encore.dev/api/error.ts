export class APIError extends Error {
  /**
   * The error code.
   */
  public readonly code: ErrCode;
  public readonly details?: ErrDetails;

  // Constructs an APIError with the Canceled error code.
  static canceled(msg: string, cause?: Error) {
    return new APIError(ErrCode.Canceled, msg, cause);
  }

  // Constructs an APIError with the Unknown error code.
  static unknown(msg: string, cause?: Error) {
    return new APIError(ErrCode.Unknown, msg, cause);
  }

  // Constructs an APIError with the InvalidArgument error code.
  static invalidArgument(msg: string, cause?: Error) {
    return new APIError(ErrCode.InvalidArgument, msg, cause);
  }

  // Constructs an APIError with the DeadlineExceeded error code.
  static deadlineExceeded(msg: string, cause?: Error) {
    return new APIError(ErrCode.DeadlineExceeded, msg, cause);
  }

  // Constructs an APIError with the NotFound error code.
  static notFound(msg: string, cause?: Error) {
    return new APIError(ErrCode.NotFound, msg, cause);
  }

  // Constructs an APIError with the AlreadyExists error code.
  static alreadyExists(msg: string, cause?: Error) {
    return new APIError(ErrCode.AlreadyExists, msg, cause);
  }

  // Constructs an APIError with the PermissionDenied error code.
  static permissionDenied(msg: string, cause?: Error) {
    return new APIError(ErrCode.PermissionDenied, msg, cause);
  }

  // Constructs an APIError with the ResourceExhausted error code.
  static resourceExhausted(msg: string, cause?: Error) {
    return new APIError(ErrCode.ResourceExhausted, msg, cause);
  }

  // Constructs an APIError with the FailedPrecondition error code.
  static failedPrecondition(msg: string, cause?: Error) {
    return new APIError(ErrCode.FailedPrecondition, msg, cause);
  }

  // Constructs an APIError with the Aborted error code.
  static aborted(msg: string, cause?: Error) {
    return new APIError(ErrCode.Aborted, msg, cause);
  }

  // Constructs an APIError with the OutOfRange error code.
  static outOfRange(msg: string, cause?: Error) {
    return new APIError(ErrCode.OutOfRange, msg, cause);
  }

  // Constructs an APIError with the Unimplemented error code.
  static unimplemented(msg: string, cause?: Error) {
    return new APIError(ErrCode.Unimplemented, msg, cause);
  }

  // Constructs an APIError with the Internal error code.
  static internal(msg: string, cause?: Error) {
    return new APIError(ErrCode.Internal, msg, cause);
  }

  // Constructs an APIError with the Unavailable error code.
  static unavailable(msg: string, cause?: Error) {
    return new APIError(ErrCode.Unavailable, msg, cause);
  }

  // Constructs an APIError with the DataLoss error code.
  static dataLoss(msg: string, cause?: Error) {
    return new APIError(ErrCode.DataLoss, msg, cause);
  }

  // Constructs an APIError with the Unauthenticated error code.
  static unauthenticated(msg: string, cause?: Error) {
    return new APIError(ErrCode.Unauthenticated, msg, cause);
  }

  // Constructs a new APIError from the previous one with the provided details
  withDetails(details: ErrDetails): APIError {
    return new APIError(this.code, this.message, this.cause as Error, details);
  }

  // Constructs an APIError with the given error code, message, and (optionally) cause.
  constructor(code: ErrCode, msg: string, cause?: Error, details?: ErrDetails) {
    // extending errors causes issues after you construct them, unless you apply the following fixes
    super(msg, { cause });
    this.code = code;
    this.details = details;

    // set error name as constructor name, make it not enumerable to keep native Error behavior
    // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
    Object.defineProperty(this, "name", {
      value: "APIError",
      enumerable: false,
      configurable: true
    });

    // Fix the prototype chain, capture stack trace.
    Object.setPrototypeOf(this, APIError.prototype);
    Error.captureStackTrace(this, this.constructor);
  }
}

export type ErrDetails = Record<string, any>;

export enum ErrCode {
  /**
   * OK indicates the operation was successful.
   */
  OK = "ok",

  /**
   * Canceled indicates the operation was canceled (typically by the caller).
   *
   * Encore will generate this error code when cancellation is requested.
   */
  Canceled = "canceled",

  /**
   * Unknown error. An example of where this error may be returned is
   * if a Status value received from another address space belongs to
   * an error-space that is not known in this address space. Also
   * errors raised by APIs that do not return enough error information
   * may be converted to this error.
   *
   * Encore will generate this error code in the above two mentioned cases.
   */
  Unknown = "unknown",

  /**
   * InvalidArgument indicates client specified an invalid argument.
   * Note that this differs from FailedPrecondition. It indicates arguments
   * that are problematic regardless of the state of the system
   * (e.g., a malformed file name).
   *
   * This error code will not be generated by the gRPC framework.
   */
  InvalidArgument = "invalid_argument",

  /**
   * DeadlineExceeded means operation expired before completion.
   * For operations that change the state of the system, this error may be
   * returned even if the operation has completed successfully. For
   * example, a successful response from a server could have been delayed
   * long enough for the deadline to expire.
   *
   * The gRPC framework will generate this error code when the deadline is
   * exceeded.
   */
  DeadlineExceeded = "deadline_exceeded",

  /**
   * NotFound means some requested entity (e.g., file or directory) was
   * not found.
   *
   * This error code will not be generated by the gRPC framework.
   */
  NotFound = "not_found",

  /**
   * AlreadyExists means an attempt to create an entity failed because one
   * already exists.
   *
   * This error code will not be generated by the gRPC framework.
   */
  AlreadyExists = "already_exists",

  /**
   * PermissionDenied indicates the caller does not have permission to
   * execute the specified operation. It must not be used for rejections
   * caused by exhausting some resource (use ResourceExhausted
   * instead for those errors). It must not be
   * used if the caller cannot be identified (use Unauthenticated
   * instead for those errors).
   *
   * This error code will not be generated by the gRPC core framework,
   * but expect authentication middleware to use it.
   */
  PermissionDenied = "permission_denied",

  /**
   * ResourceExhausted indicates some resource has been exhausted, perhaps
   * a per-user quota, or perhaps the entire file system is out of space.
   *
   * This error code will be generated by the gRPC framework in
   * out-of-memory and server overload situations, or when a message is
   * larger than the configured maximum size.
   */
  ResourceExhausted = "resource_exhausted",

  /**
   * FailedPrecondition indicates operation was rejected because the
   * system is not in a state required for the operation's execution.
   * For example, directory to be deleted may be non-empty, an rmdir
   * operation is applied to a non-directory, etc.
   *
   * A litmus test that may help a service implementor in deciding
   * between FailedPrecondition, Aborted, and Unavailable:
   *  (a) Use Unavailable if the client can retry just the failing call.
   *  (b) Use Aborted if the client should retry at a higher-level
   *      (e.g., restarting a read-modify-write sequence).
   *  (c) Use FailedPrecondition if the client should not retry until
   *      the system state has been explicitly fixed. E.g., if an "rmdir"
   *      fails because the directory is non-empty, FailedPrecondition
   *      should be returned since the client should not retry unless
   *      they have first fixed up the directory by deleting files from it.
   *  (d) Use FailedPrecondition if the client performs conditional
   *      REST Get/Update/Delete on a resource and the resource on the
   *      server does not match the condition. E.g., conflicting
   *      read-modify-write on the same resource.
   *
   * This error code will not be generated by the gRPC framework.
   */
  FailedPrecondition = "failed_precondition",

  /**
   * Aborted indicates the operation was aborted, typically due to a
   * concurrency issue like sequencer check failures, transaction aborts,
   * etc.
   *
   * See litmus test above for deciding between FailedPrecondition,
   * Aborted, and Unavailable.
   */
  Aborted = "aborted",

  /**
   * OutOfRange means operation was attempted past the valid range.
   * E.g., seeking or reading past end of file.
   *
   * Unlike InvalidArgument, this error indicates a problem that may
   * be fixed if the system state changes. For example, a 32-bit file
   * system will generate InvalidArgument if asked to read at an
   * offset that is not in the range [0,2^32-1], but it will generate
   * OutOfRange if asked to read from an offset past the current
   * file size.
   *
   * There is a fair bit of overlap between FailedPrecondition and
   * OutOfRange. We recommend using OutOfRange (the more specific
   * error) when it applies so that callers who are iterating through
   * a space can easily look for an OutOfRange error to detect when
   * they are done.
   *
   * This error code will not be generated by the gRPC framework.
   */
  OutOfRange = "out_of_range",

  /**
   * Unimplemented indicates operation is not implemented or not
   * supported/enabled in this service.
   *
   * This error code will be generated by the gRPC framework. Most
   * commonly, you will see this error code when a method implementation
   * is missing on the server. It can also be generated for unknown
   * compression algorithms or a disagreement as to whether an RPC should
   * be streaming.
   */
  Unimplemented = "unimplemented",

  /**
   * Internal errors. Means some invariants expected by underlying
   * system has been broken. If you see one of these errors,
   * something is very broken.
   *
   * This error code will be generated by the gRPC framework in several
   * internal error conditions.
   */
  Internal = "internal",

  /**
   * Unavailable indicates the service is currently unavailable.
   * This is a most likely a transient condition and may be corrected
   * by retrying with a backoff. Note that it is not always safe to retry
   * non-idempotent operations.
   *
   * See litmus test above for deciding between FailedPrecondition,
   * Aborted, and Unavailable.
   *
   * This error code will be generated by the gRPC framework during
   * abrupt shutdown of a server process or network connection.
   */
  Unavailable = "unavailable",

  /**
   * DataLoss indicates unrecoverable data loss or corruption.
   *
   * This error code will not be generated by the gRPC framework.
   */
  DataLoss = "data_loss",

  /**
   * Unauthenticated indicates the request does not have valid
   * authentication credentials for the operation.
   *
   * The gRPC framework will generate this error code when the
   * authentication metadata is invalid or a Credentials callback fails,
   * but also expect authentication middleware to generate it.
   */
  Unauthenticated = "unauthenticated"
}
