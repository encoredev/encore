/* eslint-disable */

import type { IncomingMessage, ServerResponse } from "http";

export type Method =
  | "GET"
  | "POST"
  | "PUT"
  | "DELETE"
  | "PATCH"
  | "HEAD"
  | "OPTIONS"
  | "TRACE"
  | "CONNECT";

export type Header<
  TypeOrName extends string | number | boolean = string,
  Name extends string = ""
> = TypeOrName extends string ? string : TypeOrName;

export type Query<
  TypeOrName extends string | number | boolean = string,
  Name extends string = ""
> = TypeOrName extends string ? string : TypeOrName;

export interface APIOptions {
  /**
   * The HTTP method(s) to match for this endpoint.
   * Use "*" to match any method.
   */
  method?: Method | Method[] | "*";

  /**
   * The request path to match for this endpoint.
   *
   * Use `:` to define single-segment parameters, e.g. `/users/:id`.
   * Use `*` to match any number of segments, e.g. `/files/*path`.
   *
   * If not specified, it defaults to `/<service-name>.<endpoint-name>`.
   */
  path?: string;

  /**
   * Whether or not to make this endpoint publicly accessible.
   * If false, the endpoint is only accessible from the internal network.
   *
   * Defaults to false if not specified.
   */
  expose?: boolean;

  /**
   * Whether or not the request must contain valid authentication credentials.
   * If set to true and the request is not authenticated,
   * Encore returns a 401 Unauthorized error.
   *
   * Defaults to false if not specified.
   */
  auth?: boolean;

  /**
   * The maximum body size, in bytes. If the request body exceeds this value,
   * Encore stops request processing and returns an error.
   *
   * If left unspecified it defaults to a reasonable default (currently 2MiB).
   * If set to `null`, the body size is unlimited.
   **/
  bodyLimit?: number | null;
}

type HandlerFn<Params, Response> = Params extends void
  ? () => Promise<Response>
  : (params: Params) => Promise<Response>;

export function api<
  Params extends object | void = void,
  Response extends object | void = void
>(
  options: APIOptions,
  fn: (params: Params) => Promise<Response>
): HandlerFn<Params, Response>;

export function api<
  Params extends object | void = void,
  Response extends object | void = void
>(
  options: APIOptions,
  fn: (params: Params) => Response
): HandlerFn<Params, Response>;
export function api(options: APIOptions, fn: any): typeof fn {
  return fn;
}

export type RawHandler = (req: IncomingMessage, resp: ServerResponse) => void;

api.raw = function raw(options: APIOptions, fn: RawHandler) {
  return fn;
};

export { APIError, ErrCode } from "./error";
export { Gateway, type GatewayConfig } from "./gateway";
