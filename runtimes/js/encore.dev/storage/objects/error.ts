import * as runtime from "../../internal/runtime/mod";

export class ObjectsError extends Error {
  constructor(msg: string) {
    // extending errors causes issues after you construct them, unless you apply the following fixes
    super(msg);

    // set error name as constructor name, make it not enumerable to keep native Error behavior
    // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
    Object.defineProperty(this, "name", {
      value: "ObjectsError",
      enumerable: false,
      configurable: true
    });

    // Fix the prototype chain, capture stack trace.
    Object.setPrototypeOf(this, ObjectsError.prototype);
    Error.captureStackTrace(this, this.constructor);
  }
}

export class ObjectNotFound extends ObjectsError {
  constructor(msg: string) {
    // extending errors causes issues after you construct them, unless you apply the following fixes
    super(msg);

    // set error name as constructor name, make it not enumerable to keep native Error behavior
    // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
    Object.defineProperty(this, "name", {
      value: "ObjectNotFound",
      enumerable: false,
      configurable: true
    });

    // Fix the prototype chain, capture stack trace.
    Object.setPrototypeOf(this, ObjectNotFound.prototype);
    Error.captureStackTrace(this, this.constructor);
  }
}

export class PreconditionFailed extends ObjectsError {
  constructor(msg: string) {
    // extending errors causes issues after you construct them, unless you apply the following fixes
    super(msg);

    // set error name as constructor name, make it not enumerable to keep native Error behavior
    // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
    Object.defineProperty(this, "name", {
      value: "PrecondionFailed",
      enumerable: false,
      configurable: true
    });

    // Fix the prototype chain, capture stack trace.
    Object.setPrototypeOf(this, PreconditionFailed.prototype);
    Error.captureStackTrace(this, this.constructor);
  }
}

export class InvalidArgument extends ObjectsError {
  constructor(msg: string) {
    // extending errors causes issues after you construct them, unless you apply the following fixes
    super(msg);

    // set error name as constructor name, make it not enumerable to keep native Error behavior
    // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
    Object.defineProperty(this, "name", {
      value: "InvalidArgument",
      enumerable: false,
      configurable: true
    });

    // Fix the prototype chain, capture stack trace.
    Object.setPrototypeOf(this, InvalidArgument.prototype);
    Error.captureStackTrace(this, this.constructor);
  }
}

export function unwrapErr<T>(val: T | runtime.TypedObjectError): T {
  if (val instanceof runtime.TypedObjectError) {
    switch (val.kind) {
      case runtime.ObjectErrorKind.NotFound:
        throw new ObjectNotFound(val.message);
      case runtime.ObjectErrorKind.PreconditionFailed:
        throw new PreconditionFailed(val.message);
      case runtime.ObjectErrorKind.InvalidArgument:
        throw new InvalidArgument(val.message);
      default:
        throw new ObjectsError(val.message);
    }
  }

  return val;
}
