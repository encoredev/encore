/* eslint-disable */
import { Loc, Decl, Type } from "../../../../encore/parser/schema/v1/schema.pb";

export const protobufPackage = "encore.parser.meta.v1";

/** Data is the metadata associated with an app version. */
export interface Data {
  /** app module path */
  module_path: string;
  app_version: string;
  decls: Decl[];
  pkgs: Package[];
  svcs: Service[];
  /** the auth handler or nil */
  auth_handler?: AuthHandler | undefined;
}

/**
 * QualifiedName is a name of an object in a specific package.
 * It is never an unqualified name, even in circumstances
 * where a package may refer to its own objects.
 */
export interface QualifiedName {
  /** "rel/path/to/pkg" */
  pkg: string;
  /** ObjectName */
  name: string;
}

export interface Package {
  /** import path relative to app root ("." for the app root itself) */
  rel_path: string;
  /** package name as declared in Go files */
  name: string;
  /** associated documentation */
  doc: string;
  /** service name this package is a part of, if any */
  service_name: string;
  /** secrets required by this package */
  secrets: string[];
  /** RPCs called by the package */
  rpc_calls: QualifiedName[];
  trace_nodes: TraceNode[];
}

export interface Service {
  name: string;
  /** import path relative to app root for the root package in the service */
  rel_path: string;
  rpcs: RPC[];
  migrations: DBMigration[];
}

export interface DBMigration {
  /** filename */
  filename: string;
  /** migration number */
  number: number;
  /** descriptive name */
  description: string;
}

export interface RPC {
  /** name of the RPC endpoint */
  name: string;
  /** associated documentation */
  doc: string;
  /** the service the RPC belongs to. */
  service_name: string;
  /** how can the RPC be accessed? */
  access_type: RPC_AccessType;
  /** request schema, or nil */
  request_schema?: Type | undefined;
  /** response schema, or nil */
  response_schema?: Type | undefined;
  proto: RPC_Protocol;
  loc: Loc;
  path: Path;
  http_methods: string[];
}

export enum RPC_AccessType {
  PRIVATE = "PRIVATE",
  PUBLIC = "PUBLIC",
  AUTH = "AUTH",
  UNRECOGNIZED = "UNRECOGNIZED",
}

export enum RPC_Protocol {
  REGULAR = "REGULAR",
  RAW = "RAW",
  UNRECOGNIZED = "UNRECOGNIZED",
}

export interface AuthHandler {
  name: string;
  doc: string;
  /** package (service) import path */
  pkg_path: string;
  /** package (service) name */
  pkg_name: string;
  loc: Loc;
  /** custom auth data, or nil */
  auth_data?: Type | undefined;
}

export interface TraceNode {
  id: number;
  /** slash-separated, relative to app root */
  filepath: string;
  start_pos: number;
  end_pos: number;
  src_line_start: number;
  src_line_end: number;
  src_col_start: number;
  src_col_end: number;
  rpc_def: RPCDefNode | undefined;
  rpc_call: RPCCallNode | undefined;
  static_call: StaticCallNode | undefined;
  auth_handler_def: AuthHandlerDefNode | undefined;
}

export interface RPCDefNode {
  service_name: string;
  rpc_name: string;
  context: string;
}

export interface RPCCallNode {
  service_name: string;
  rpc_name: string;
  context: string;
}

export interface StaticCallNode {
  package: StaticCallNode_Package;
  func: string;
  context: string;
}

export enum StaticCallNode_Package {
  UNKNOWN = "UNKNOWN",
  SQLDB = "SQLDB",
  RLOG = "RLOG",
  UNRECOGNIZED = "UNRECOGNIZED",
}

export interface AuthHandlerDefNode {
  service_name: string;
  name: string;
  context: string;
}

export interface Path {
  segments: PathSegment[];
}

export interface PathSegment {
  type: PathSegment_SegmentType;
  value: string;
  value_type: PathSegment_ParamType;
}

export enum PathSegment_SegmentType {
  LITERAL = "LITERAL",
  PARAM = "PARAM",
  WILDCARD = "WILDCARD",
  UNRECOGNIZED = "UNRECOGNIZED",
}

export enum PathSegment_ParamType {
  STRING = "STRING",
  BOOL = "BOOL",
  INT8 = "INT8",
  INT16 = "INT16",
  INT32 = "INT32",
  INT64 = "INT64",
  INT = "INT",
  UINT8 = "UINT8",
  UINT16 = "UINT16",
  UINT32 = "UINT32",
  UINT64 = "UINT64",
  UINT = "UINT",
  UUID = "UUID",
  UNRECOGNIZED = "UNRECOGNIZED",
}
