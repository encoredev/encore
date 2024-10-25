/* eslint-disable */

import type { IncomingMessage, ServerResponse } from "http";
import { RequestMeta } from "../mod";
export { RawRequest, RawResponse } from "../internal/api/node_http";

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
  TypeOrName extends string | number | boolean | Date = string,
  Name extends string = ""
> = TypeOrName extends string ? string : TypeOrName;

export type Query<
  TypeOrName extends string | number | boolean | Date = string,
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

export interface StreamOptions {
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

export interface StreamIn<Request> extends AsyncIterable<Request> {
  recv: () => Promise<Request>;
}
export interface StreamOutWithResponse<Request, Response>
  extends StreamOut<Request> {
  response: () => Promise<Response>;
}

export interface StreamOut<Response> {
  send: (msg: Response) => Promise<void>;
  close: () => Promise<void>;
}

export type StreamInOutHandlerFn<HandshakeData, Request, Response> =
  HandshakeData extends void
    ? (stream: StreamInOut<Request, Response>) => Promise<void>
    : (
        data: HandshakeData,
        stream: StreamInOut<Request, Response>
      ) => Promise<void>;

export type StreamOutHandlerFn<HandshakeData, Response> =
  HandshakeData extends void
    ? (stream: StreamOut<Response>) => Promise<void>
    : (data: HandshakeData, stream: StreamOut<Response>) => Promise<void>;

export type StreamInHandlerFn<HandshakeData, Request, Response> =
  HandshakeData extends void
    ? (stream: StreamIn<Request>) => Promise<Response>
    : (data: HandshakeData, stream: StreamIn<Request>) => Promise<Response>;

export type StreamInOut<Request, Response> = StreamIn<Request> &
  StreamOut<Response>;

function streamInOut<HandshakeData, Request, Response>(
  options: StreamOptions,
  fn: (
    data: HandshakeData,
    stream: StreamInOut<Request, Response>
  ) => Promise<void>
): StreamInOutHandlerFn<HandshakeData, Request, Response>;
function streamInOut<Request, Response>(
  options: StreamOptions,
  fn: (stream: StreamInOut<Request, Response>) => Promise<void>
): StreamInOutHandlerFn<void, Request, Response>;
function streamInOut(options: StreamOptions, fn: any): typeof fn {
  return fn;
}

function streamIn<Request>(
  options: StreamOptions,
  fn: (stream: StreamIn<Request>) => Promise<void>
): StreamInHandlerFn<void, Request, void>;
function streamIn<Request, Response>(
  options: StreamOptions,
  fn: (stream: StreamIn<Request>) => Promise<Response>
): StreamInHandlerFn<void, Request, Response>;
function streamIn<HandshakeData, Request, Response>(
  options: StreamOptions,
  fn: (data: HandshakeData, stream: StreamIn<Request>) => Promise<Response>
): StreamInHandlerFn<HandshakeData, Request, Response>;
function streamIn(options: StreamOptions, fn: any): typeof fn {
  return fn;
}

function streamOut<HandshakeData, Response>(
  options: StreamOptions,
  fn: (data: HandshakeData, stream: StreamOut<Response>) => Promise<void>
): StreamOutHandlerFn<HandshakeData, Response>;
function streamOut<Response>(
  options: StreamOptions,
  fn: (stream: StreamOut<Response>) => Promise<void>
): StreamOutHandlerFn<void, Response>;
function streamOut(options: StreamOptions, fn: any): typeof fn {
  return fn;
}

api.streamInOut = streamInOut;
api.streamIn = streamIn;
api.streamOut = streamOut;

export interface StaticOptions {
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
   * The relative path to the directory containing the static files to serve.
   *
   * The provided path must be a subdirectory from the calling file's directory.
   */
  dir: string;

  /**
   * Path to the file to serve when the requested file is not found.
   * The path is relative to `dir` and must exist within that directory.
   */
  notFound?: string;
}

export class StaticAssets {
  public readonly options: StaticOptions;

  constructor(options: StaticOptions) {
    this.options = options;
  }
}

api.static = function staticAssets(options: StaticOptions) {
  return new StaticAssets(options);
};

export interface MiddlewareOptions {
  /**
   * If set, only run middleware on endpoints that are either exposed or not
   * exposed.
   */
  exposed?: boolean;

  /**
   * If set, only run middleware on endpoints that either require on not
   * requires auth.
   */
  requiresAuth?: boolean;
}

export type Next = (req: RequestMeta) => Promise<any>;
export interface Middleware {
  (req: RequestMeta, next: Next): Promise<any>;
  options?: MiddlewareOptions;
}

export function middleware(m: Middleware): Middleware;
export function middleware(
  options: MiddlewareOptions,
  fn: Middleware
): Middleware;

export function middleware(...args: unknown[]): Middleware {
  if (args.length > 1) {
    let m = args[1] as Middleware;
    let o = args[0] as MiddlewareOptions;
    m.options = o;
    return m;
  }
  return args[0] as Middleware;
}

export { APIError, ErrCode } from "./error";
export { Gateway, type GatewayConfig } from "./gateway";
