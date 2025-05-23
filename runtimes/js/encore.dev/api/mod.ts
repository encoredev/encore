/* eslint-disable */

import type { IncomingMessage, ServerResponse } from "http";
import { RequestMeta, currentRequest } from "../mod";
import { RawResponse } from "./mod";
import { RawRequest } from "./mod";
import { InternalHandlerResponse } from "../internal/appinit/mod";
import { IterableSocket, IterableStream, Sink } from "./stream";
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
  TypeOrName extends
    | string
    | string[]
    | number
    | number[]
    | boolean
    | boolean[]
    | Date
    | Date[] = string,
  Name extends string = ""
> = TypeOrName extends string ? string : TypeOrName;

export type CookieWithOptions<T> = {
  value: T;
  expires?: Date;
  sameSite?: "Strict" | "Lax" | "None";
  domain?: string;
  path?: string;
  maxAge?: number;
  secure?: boolean;
  httpOnly?: boolean;
  partitioned?: boolean;
};

export type Cookie<
  TypeOrName extends string | number | boolean | Date = string,
  Name extends string = ""
> = TypeOrName extends string
  ? CookieWithOptions<string>
  : CookieWithOptions<TypeOrName>;

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

  /**
   * Tags to filter endpoints when generating clients and in middlewares.
   */
  tags?: string[];
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
   * The path must be a relative path to within the calling file's directory.
   */
  notFound?: string;

  /**
   * Http Status code used when serving notFound fallback.
   * Defaults to 404.
   */
  notFoundStatus?: number;
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
   * Configuration for what endpoints that should be targeted by the middleware
   */
  target?: {
    /**
     * If set, only run middleware on endpoints that are either exposed or not
     * exposed.
     */
    expose?: boolean;

    /**
     * If set, only run middleware on endpoints that either require or not
     * requires auth.
     */
    auth?: boolean;

    /**
     * If set, only run middleware on endpoints that are raw endpoints.
     */
    isRaw?: boolean;

    /**
     * If set, only run middleware on endpoints that are stream endpoints.
     */
    isStream?: boolean;

    /**
     * If set, only run middleware on endpoints that have specific tags.
     * These tags are evaluated with OR, meaning the middleware applies to an
     * API if the API has at least one of those tags.
     */
    tags?: string[];
  };
}

export class MiddlewareRequest {
  private _reqMeta?: RequestMeta;
  private _stream?: IterableStream | IterableSocket | Sink;
  private _rawReq?: RawRequest;
  private _rawResp?: RawResponse;
  private _data?: Record<string, any>;

  constructor(
    stream?: IterableStream | IterableSocket | Sink,
    rawReq?: RawRequest,
    rawResp?: RawResponse
  ) {
    this._stream = stream;
    this._rawReq = rawReq;
    this._rawResp = rawResp;
  }

  /**
   * requestMeta is set when the handler is a typed handler or a stream handler.
   * for raw handlers, see rawRequest and rawResponse.
   */
  public get requestMeta(): RequestMeta | undefined {
    return this._reqMeta || (this._reqMeta = currentRequest());
  }

  /**
   * stream is set when the handler is a stream handler.
   */
  public get stream(): IterableStream | IterableSocket | Sink | undefined {
    return this._stream;
  }

  /**
   * rawRequest is set when the handler is a raw request handler.
   */
  public get rawRequest(): RawRequest | undefined {
    return this._rawReq;
  }

  /**
   * rawResponse is set when the handler is a raw request handler.
   */
  public get rawResponse(): RawResponse | undefined {
    return this._rawResp;
  }

  /**
   * data can be used to pass data from middlewares to the handler.
   * The data will be available via `currentRequest()`
   */
  public get data(): Record<string, any> {
    if (this._data === undefined) {
      this._data = {};
    }
    return this._data;
  }
}

export class ResponseHeader {
  headers: Record<string, string | string[]>;

  constructor() {
    this.headers = {};
  }

  /**
   * set will set a header value for a key, if a previous middleware has
   * already set a value, it will be overridden.
   */
  public set(key: string, value: string | string[]) {
    this.headers[key] = value;
  }

  /**
   * add adds a header value to a key, if a previous middleware has
   * already set a value, they will be appended.
   */
  public add(key: string, value: string | string[]) {
    const prev = this.headers[key];

    if (prev === undefined) {
      this.headers[key] = value;
    } else {
      this.headers[key] = [prev, value].flat();
    }
  }
}

export class HandlerResponse {
  /**
   * The payload returned by the handler when the handler is either
   * a typed handler or stream handler.
   */
  payload: any;

  private _headers?: ResponseHeader;
  private _status?: number;

  constructor(payload: any) {
    this.payload = payload;
  }

  /**
   * header can be used by middlewares to set headers to the
   * response. This only works for typed handler. For raw handlers
   * see MiddlewareRequest.rawResponse.
   */
  public get header(): ResponseHeader {
    if (this._headers === undefined) {
      this._headers = new ResponseHeader();
    }

    return this._headers;
  }

  /**
   * Override the http status code for successful requests for typed endpoints.
   */
  public set status(s: number) {
    this._status = s;
  }

  /**
   * __internalToResponse converts a response to the internal representation
   */
  __internalToResponse(): InternalHandlerResponse {
    return {
      payload: this.payload,
      extraHeaders: this._headers?.headers,
      status: this._status
    };
  }
}

export type Next = (req: MiddlewareRequest) => Promise<HandlerResponse>;

export type MiddlewareFn = (
  req: MiddlewareRequest,
  next: Next
) => Promise<HandlerResponse>;

export interface Middleware extends MiddlewareFn {
  options?: MiddlewareOptions;
}

export function middleware(m: MiddlewareFn): Middleware;
export function middleware(
  options: MiddlewareOptions,
  fn: MiddlewareFn
): Middleware;

export function middleware(
  a: MiddlewareFn | MiddlewareOptions,
  b?: MiddlewareFn
): Middleware {
  if (b === undefined) {
    return a as Middleware;
  } else {
    const opts = a as MiddlewareOptions;
    // Wrap the middleware function to delegate calls and preserve the original options.
    // The options object is stored separately and made immutable to prevent accidental mutation.
    const mw: Middleware = (req: MiddlewareRequest, next: Next) => {
      return b(req, next);
    };
    mw.options = Object.freeze({ ...opts });

    return mw;
  }
}

/**
 * Options when making api calls.
 *
 * This interface will be extended with additional fields from
 * app's generated code.
 */
export interface CallOpts {
  /* authData?: AuthData */
}

export { APIError, ErrCode } from "./error";
export { Gateway, type GatewayConfig } from "./gateway";
export { IterableSocket, IterableStream, Sink } from "./stream";
