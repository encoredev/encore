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
  method?: Method | Method[];
  path?: string;
  expose?: boolean;
  auth?: boolean;
}

export type Handler<
  Params extends object | void = void,
  Response extends object | void = void
> = Params extends void
  ? () => Promise<Response>
  : (params: Params) => Promise<Response>;

export function api<
  Params extends object | void = void,
  Response extends object | void = void
>(
  options: APIOptions,
  fn: Handler<Params, Response>
): Handler<Params, Response> {
  return fn;
}

export type RawHandler = (req: IncomingMessage, resp: ServerResponse) => void;

api.raw = function raw(options: APIOptions, fn: RawHandler) {
  return fn;
};

export { APIError, ErrCode } from "./error";
export { Gateway, type GatewayConfig } from "./gateway";
