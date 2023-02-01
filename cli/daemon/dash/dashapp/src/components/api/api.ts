import * as pb from "../../../../../../../proto/encore/parser/meta/v1/meta.pb";

export * from "../../../../../../../proto/encore/parser/meta/v1/meta.pb";

// Aliases to match old type names
export type APIMeta = pb.Data;

export interface ParameterEncoding {
  name: string;
  location: "body" | "query" | "header" | "undefined";
  omit_empty: boolean;
  src_name: string;
  doc: string;
  raw_tag: string;
  wire_format: string;
}

export interface AuthEncoding {
  header_parameters: ParameterEncoding[] | null;
  query_parameters: ParameterEncoding[] | null;
}

export interface RPCEncoding {
  name: string;
  doc: string;
  access_type: string;
  proto: string;
  path: {
    segments: {
      value: string;
      type?: number;
      value_type?: number;
    }[];
  };
  http_methods: string[] | null;
  default_method: "GET";
  request_encoding: {
    http_methods: string[];
    header_parameters: ParameterEncoding[] | null;
    query_parameters: ParameterEncoding[] | null;
    body_parameters: ParameterEncoding[] | null;
  };
  all_request_encodings: {
    http_methods: string[];
    header_parameters: ParameterEncoding[] | null;
    query_parameters: ParameterEncoding[] | null;
    body_parameters: ParameterEncoding[] | null;
  }[];
  response_encoding: {
    header_parameters: ParameterEncoding[] | null;
    body_parameters: ParameterEncoding[] | null;
  };
}

export interface ServiceEncoding {
  name: string;
  doc: string;
  rpcs: RPCEncoding[];
}

export interface APIEncoding {
  authorization: AuthEncoding | null;
  services: ServiceEncoding[];
}
