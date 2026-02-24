import * as runtime from "../internal/runtime/mod";
import { getCurrentRequest } from "../internal/reqtrack/mod";

/**
 * Options for configuring a custom trace span.
 */
export interface TraceSpanOptions {
  /**
   * The name of the span. This is displayed in the trace viewer.
   */
  name: string;

  /**
   * Optional key-value attributes to attach to the span.
   * These are recorded at span start and can be used for filtering/searching.
   */
  attributes?: Record<string, string>;
}

/**
 * Wraps a function with a custom trace span. When the wrapped function is called,
 * a span is started before execution and ended when the function completes
 * (or throws an error).
 *
 * The span is recorded as part of the current request's trace. If there is no
 * active request (i.e. the function is called outside a request context), the
 * function executes normally without any tracing.
 *
 * @example
 * ```typescript
 * import { trace } from "encore.dev/tracing";
 *
 * const processOrder = trace(
 *   { name: "processOrder", attributes: { priority: "high" } },
 *   async (orderId: string) => {
 *     // ... your logic here
 *     return { success: true };
 *   }
 * );
 *
 * // Call it like a normal function:
 * const result = await processOrder("order-123");
 * ```
 */
export function trace<Args extends any[], R>(
  options: TraceSpanOptions,
  fn: (...args: Args) => Promise<R>
): (...args: Args) => Promise<R>;

export function trace<Args extends any[], R>(
  options: TraceSpanOptions,
  fn: (...args: Args) => R
): (...args: Args) => R;

export function trace<Args extends any[], R>(
  options: TraceSpanOptions,
  fn: (...args: Args) => R | Promise<R>
): (...args: Args) => R | Promise<R> {
  const attrs = options.attributes ?? {};

  return function tracedFn(this: any, ...args: Args): R | Promise<R> {
    const req = getCurrentRequest();
    if (!req) {
      // No active request context, just call the function directly.
      return fn.apply(this, args);
    }

    const startEventId = runtime.RT.customSpanStart(
      req as any,
      options.name,
      attrs
    );
    if (!startEventId) {
      // Request is not being traced, just call the function directly.
      return fn.apply(this, args);
    }

    let result: R | Promise<R>;
    try {
      result = fn.apply(this, args);
    } catch (err) {
      runtime.RT.customSpanEnd(
        req as any,
        startEventId,
        err instanceof Error ? err.message : String(err)
      );
      throw err;
    }

    // If the result is a promise, end the span when it resolves/rejects.
    if (result instanceof Promise) {
      return result.then(
        (value) => {
          runtime.RT.customSpanEnd(req as any, startEventId, null);
          return value;
        },
        (err) => {
          runtime.RT.customSpanEnd(
            req as any,
            startEventId,
            err instanceof Error ? err.message : String(err)
          );
          throw err;
        }
      ) as Promise<R> as any;
    }

    // Synchronous result - end the span immediately.
    runtime.RT.customSpanEnd(req as any, startEventId, null);
    return result;
  };
}
