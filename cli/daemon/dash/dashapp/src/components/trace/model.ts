import { Base64EncodedBytes } from "~lib/base64"

export interface Trace {
  id: string;
  app_version: string;
  date: string;
  start_time: number;
  end_time?: number;
  root: Request;
  auth: Request | null;
  locations: {[key: number]: TraceExpr};
}

export interface Request {
  type: "RPC" | "AUTH";
  id: string;
  parent_id: string | null;
  goid: number;
  caller_goid: number | null;
  def_loc: number;
  call_loc: number | null;
  start_time: number;
  end_time?: number;
  inputs: Base64EncodedBytes[];
  outputs: Base64EncodedBytes[];
  err: Base64EncodedBytes | null;
  events: Event[];
  children: Request[];
}

export interface DBTransaction {
  type: "DBTransaction";
  goid: number;
  txid: number;
  start_loc: number;
  end_loc: number;
  start_time: number;
  end_time?: number;
  completion_type: "COMMIT" | "ROLLBACK";
  err: Base64EncodedBytes | null;
  queries: DBQuery[];
}

export interface DBQuery {
  type: "DBQuery";
  goid: number;
  txid: number | null;
  call_loc: number;
  start_time: number;
  end_time?: number;
  query: Base64EncodedBytes;
  html_query: Base64EncodedBytes | null;
  err: Base64EncodedBytes | null;
}

export interface Goroutine {
  type: "Goroutine";
  goid: number;
  call_loc: number;
  start_time: number;
  end_time?: number;
}

export interface RPCCall {
  type: "RPCCall";
  goid: number;
  req_id: string;
  call_loc: number;
  def_loc: number;
  start_time: number;
  end_time?: number;
  err: Base64EncodedBytes | null;
}

export interface AuthCall {
  type: "AuthCall";
  goid: number;
  def_loc: number;
  start_time: number;
  end_time?: number;
  uid: string;
  auth_data: Base64EncodedBytes | null;
  err: Base64EncodedBytes | null;
}

export interface HTTPCall {
  type: "HTTPCall";
  goid: number;
  req_id: string;
  start_time: number;
  end_time?: number;
  method: string;
  host: string;
  path: string;
  url: string;
  status_code: number;
  err: Base64EncodedBytes | null;
  metrics: HTTPCallMetrics;
}

export interface HTTPCallMetrics {
  got_conn?: number;
  conn_reused: boolean;
  dns_done?: number;
  tls_handshake_done?: number;
  wrote_headers?: number;
  wrote_request?: number;
  first_response?: number;
  body_closed?: number;
}

export type Event = DBTransaction | DBQuery | RPCCall | HTTPCall | Goroutine;

export type TraceExpr = RpcDefExpr | RpcCallExpr | StaticCallExpr | AuthHandlerDefExpr

interface BaseExpr {
  filepath: string;       // source file path
  src_line_start: number; // line number in source file
  src_line_end: number;   // line number in source file
  src_col_start: number;  // column start in source file
  src_col_end: number;    // colum end in source file (exclusive)
}

type RpcDefExpr = BaseExpr & {
  rpc_def: {
    service_name: string;
    rpc_name: string;
    context: string;
  }
}

type RpcCallExpr = BaseExpr & {
  rpc_call: {
    service_name: string;
    rpc_name: string;
    context: string;
  }
}

type StaticCallExpr = BaseExpr & {
  static_call: {
    package: "SQLDB" | "RLOG";
    func: string;
  }
}

type AuthHandlerDefExpr = BaseExpr & {
  auth_handler_def: {
    service_name: string;
    name: string;
    context: string;
  }
}