import {APIMeta} from "~c/api/api";
import {Base64EncodedBytes} from "~lib/base64"

export interface Trace {
  id: string;
  app_version: string;
  date: string;
  start_time: number;
  end_time?: number;
  root: Request | null;
  auth: Request | null;
  locations: {[key: number]: TraceExpr};
  meta: APIMeta;
}

export interface Request {
  type: "RPC" | "AUTH" | "PUBSUB_MSG";
  id: string;
  parent_id: string | null;
  goid: number;
  caller_goid: number | null;

  svc_name: string;
  rpc_name: string;
  topic_name: string;
  subscriber_name: string;
  msg_id: string;
  attempt: number;
  published: number | null;

  def_loc: number;
  call_loc: number | null;

  start_time: number;
  end_time?: number;
  inputs: Base64EncodedBytes[];
  outputs: Base64EncodedBytes[];
  err: Base64EncodedBytes | null;
  err_stack: Stack | null;
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
  begin_stack: Stack;
  end_stack: Stack | null;
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
  stack: Stack;
}

export interface Goroutine {
  type: "Goroutine";
  goid: number;
  call_loc: number;
  start_time: number;
  end_time?: number;
}

export interface PubSubPublish {
  type: "PubSubPublish";
  goid: number;
  start_time: number;
  end_time?: number;
  topic: string;
  message: Base64EncodedBytes;
  message_id?: string;
  err: Base64EncodedBytes | null;
  stack: Stack;
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
  stack: Stack;
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
  stack: Stack;
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

export interface LogMessage {
  type: "LogMessage";
  goid: number;
  time: number;
  level: "DEBUG" | "INFO" | "ERROR";
  msg: string;
  fields: LogField[];
  stack: Stack;
}

export interface LogField {
  key: string;
  value: any;
  stack: Stack | null;
}

export interface Stack {
  frames: StackFrame[];
}

export interface StackFrame {
  short_file: string;
  full_file: string
  func: string;
  line: number;
}

export type Event = DBTransaction | DBQuery | RPCCall | HTTPCall | Goroutine | LogMessage | PubSubPublish;

export type TraceExpr = RpcDefExpr | RpcCallExpr | StaticCallExpr | AuthHandlerDefExpr | PubsubSubscriberExpr

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

type PubsubSubscriberExpr = BaseExpr & {
  pubsub_subscriber: {
    service_name: string;
    topic_name: string;
    subscriber_name: string;
    context: string;
  }
}
