import * as runtime from "../internal/runtime/mod";
import { StringLiteral } from "../internal/utils/constraints";

/**
 * Secret represents a single secret value that is loaded
 * into the application. It is strongly typed for that secret,
 * so that you can write functions which expect a specific one.
 *
 * You can use {@link AnySecret} to represent any secret without knowing
 * it's name.
 *
 * @example
 *
 * function doFoo(s: Secret<"foo">): void {
 *   const foo = s();
 * }
 */
export interface Secret<Name extends string> {
  /**
   * Returns the current value of the secret.
   *
   * Encore will periodically refresh the value of the secret, so this
   * value may change over time and could be stale for upto a couple of
   * minutes. If you need to ensure you have the latest value, use
   * {@link latest}.
   */
  (): string;

  /**
   * The name of the secret.
   */
  readonly name: Name;
}

/**
 * AnySecret is a type which can be used to represent any {@link Secret}
 * without knowing its name.
 */
export type AnySecret = Secret<string>;

/**
 * secret is used to load a single {@link Secret} into the application.
 *
 * If you wish to load multiple secrets at once, see {@link secrets}.
 *
 * @example loading a single secret
 *  import {secret} from "encore.dev/config/secrets";
 *  const foo = secret<"foo">();
 */
export function secret<Name extends string>(
  name: StringLiteral<Name>,
): Secret<Name> {
  // Get the secret implementation from the runtime.
  // It reports null if the secret isn't in the runtime config.
  const impl = runtime.RT.secret(name);
  const secretObj = () => {
    if (impl === null) {
      throw new Error(`secret ${name} is not set`);
    }
    return impl.cached();
  };

  secretObj.toString = () => {
    if (impl === null) {
      return `Secret<${name}>(not set)`;
    }
    return `Secret<${name}>(*********)`;
  };
  Object.defineProperty(secretObj, "name", { value: name });

  return secretObj as unknown as Secret<Name>;
}
