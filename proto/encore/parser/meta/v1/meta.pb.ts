/* eslint-disable */
import type {
  Loc,
  Type,
  Builtin,
  Decl,
} from "../../../../encore/parser/schema/v1/schema.pb";

export const protobufPackage = "encore.parser.meta.v1";

/** Data is the metadata associated with an app version. */
export interface Data {
  /** app module path */
  module_path: string;
  /** app revision (always the VCS revision reference) */
  app_revision: string;
  /** true if there where changes made on-top of the VCS revision */
  uncommitted_changes: boolean;
  decls: Decl[];
  pkgs: Package[];
  svcs: Service[];
  /** the auth handler or nil */
  auth_handler?: AuthHandler | undefined;
  cron_jobs: CronJob[];
  /** All the pub sub topics declared in the application */
  pubsub_topics: PubSubTopic[];
  middleware: Middleware[];
  cache_clusters: CacheCluster[];
  experiments: string[];
  metrics: Metric[];
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
  /** databases this service connects to */
  databases: string[];
  /** true if the service has uses config */
  has_config: boolean;
}

export interface Selector {
  type: Selector_Type;
  value: string;
}

export enum Selector_Type {
  UNKNOWN = "UNKNOWN",
  ALL = "ALL",
  /** TAG - NOTE: If more types are added, update the (selector.Selector).ToProto method. */
  TAG = "TAG",
  UNRECOGNIZED = "UNRECOGNIZED",
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
  tags: Selector[];
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
  /** builtin string or named type */
  params?: Type | undefined;
}

export interface Middleware {
  name: QualifiedName;
  doc: string;
  loc: Loc;
  global: boolean;
  /** nil if global */
  service_name?: string | undefined;
  target: Selector[];
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
  pubsub_topic_def: PubSubTopicDefNode | undefined;
  pubsub_publish: PubSubPublishNode | undefined;
  pubsub_subscriber: PubSubSubscriberNode | undefined;
  service_init: ServiceInitNode | undefined;
  middleware_def: MiddlewareDefNode | undefined;
  cache_keyspace: CacheKeyspaceDefNode | undefined;
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

export interface PubSubTopicDefNode {
  topic_name: string;
  context: string;
}

export interface PubSubPublishNode {
  topic_name: string;
  context: string;
}

export interface PubSubSubscriberNode {
  topic_name: string;
  subscriber_name: string;
  service_name: string;
  context: string;
}

export interface ServiceInitNode {
  service_name: string;
  setup_func_name: string;
  context: string;
}

export interface MiddlewareDefNode {
  pkg_rel_path: string;
  name: string;
  context: string;
  target: Selector[];
}

export interface CacheKeyspaceDefNode {
  pkg_rel_path: string;
  var_name: string;
  cluster_name: string;
  context: string;
}

export interface Path {
  segments: PathSegment[];
  type: Path_Type;
}

export enum Path_Type {
  URL = "URL",
  CACHE_KEYSPACE = "CACHE_KEYSPACE",
  UNRECOGNIZED = "UNRECOGNIZED",
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
  FALLBACK = "FALLBACK",
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

export interface CronJob {
  id: string;
  title: string;
  doc: string;
  schedule: string;
  endpoint: QualifiedName;
}

export interface PubSubTopic {
  /** The pub sub topic name (unique per application) */
  name: string;
  /** The documentation for the topic */
  doc: string;
  /** The type of the message */
  message_type: Type;
  /** The delivery guarantee for the topic */
  delivery_guarantee: PubSubTopic_DeliveryGuarantee;
  /** The field used to group messages; if empty, the topic is not ordered */
  ordering_key: string;
  /** The publishers for this topic */
  publishers: PubSubTopic_Publisher[];
  /** The subscriptions to the topic */
  subscriptions: PubSubTopic_Subscription[];
}

export enum PubSubTopic_DeliveryGuarantee {
  /** AT_LEAST_ONCE - All messages will be delivered to each subscription at least once */
  AT_LEAST_ONCE = "AT_LEAST_ONCE",
  /** EXACTLY_ONCE - All messages will be delivered to each subscription exactly once */
  EXACTLY_ONCE = "EXACTLY_ONCE",
  UNRECOGNIZED = "UNRECOGNIZED",
}

export interface PubSubTopic_Publisher {
  /** The service the publisher is in */
  service_name: string;
}

export interface PubSubTopic_Subscription {
  /** The unique name of the subscription for this topic */
  name: string;
  /** The service that the subscriber is in */
  service_name: string;
  /** How long has a consumer got to process and ack a message in nanoseconds */
  ack_deadline: number;
  /** How long is an undelivered message kept in nanoseconds */
  message_retention: number;
  /** The retry policy for the subscription */
  retry_policy: PubSubTopic_RetryPolicy;
}

export interface PubSubTopic_RetryPolicy {
  /** min backoff in nanoseconds */
  min_backoff: number;
  /** max backoff in nanoseconds */
  max_backoff: number;
  /** max number of retries */
  max_retries: number;
}

export interface CacheCluster {
  /** The pub sub topic name (unique per application) */
  name: string;
  /** The documentation for the topic */
  doc: string;
  /** The publishers for this topic */
  keyspaces: CacheCluster_Keyspace[];
  /** redis eviction policy */
  eviction_policy: string;
}

export interface CacheCluster_Keyspace {
  key_type: Type;
  value_type: Type;
  service: string;
  doc: string;
  path_pattern: Path;
}

export interface Metric {
  /** the name of the metric */
  name: string;
  value_type: Builtin;
  /** the doc string */
  doc: string;
  kind: Metric_MetricKind;
  /** the service the metric is exclusive to, if any. */
  service_name?: string | undefined;
  labels: Metric_Label[];
}

export enum Metric_MetricKind {
  COUNTER = "COUNTER",
  GAUGE = "GAUGE",
  HISTOGRAM = "HISTOGRAM",
  UNRECOGNIZED = "UNRECOGNIZED",
}

export interface Metric_Label {
  key: string;
  type: Builtin;
  doc: string;
}
