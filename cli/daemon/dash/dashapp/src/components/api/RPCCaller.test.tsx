import React from "react";
import { copyAsCurlToClipboard } from "~c/api/RPCCaller";
import { APIEncoding, RPC } from "~c/api/api";

(document as any).execCommand = function execCommandMock() {};

describe("RPCCaller", () => {
  describe("copyAsCurlToClipboard", () => {
    const serializeRequest = {
      path: "/path",
      reqBody: `
          {
              // HTTP headers
              "X-Alpha": "some string",
              "X-Delta": "some string",
          
              // Query string
              "beta": [1, 2],
              "echo": true,
          
              // HTTP body
              "charile": "some string"
          }
        `,
      authBody: `
          // Authentication Data
          {
              // HTTP headers
              "X-Client-ID": "some string",
          
              // Query string
              "key": "some string"
          }
        `,
    };

    const rpcMock = {
      request_schema: { named: { id: 123 } },
      name: "rpc-name",
      service_name: "service-name",
    } as RPC;

    const apiEncodingMock = {
      authorization: {
        query_parameters: [
          {
            name: "key",
            location: "query",
          } as any,
        ],
        header_parameters: [
          {
            name: "X-Client-ID",
            location: "header",
          } as any,
        ],
      },
      services: [
        {
          name: "service-name",
          rpcs: [
            {
              name: "rpc-name",
              all_request_encodings: [
                {
                  http_methods: ["POST"],
                  header_parameters: [
                    {
                      name: "X-Alpha",
                      location: "header",
                    },
                    {
                      name: "X-Delta",
                      location: "header",
                    },
                  ] as any,
                  query_parameters: [
                    {
                      name: "beta",
                      location: "query",
                    },
                    {
                      name: "echo",
                      location: "query",
                    },
                  ] as any,
                  body_parameters: [
                    {
                      name: "charile",
                      location: "body",
                    } as any,
                  ],
                },
                {
                  http_methods: ["GET"],
                  header_parameters: [
                    {
                      name: "X-Alpha",
                      location: "header",
                    },
                  ] as any,
                  query_parameters: [
                    {
                      name: "echo",
                      location: "query",
                    },
                  ] as any,
                  body_parameters: null,
                },
              ],
            },
          ],
        },
      ],
    } as APIEncoding;

    it("should create curl for POST request", () => {
      const result = copyAsCurlToClipboard({
        serializeRequest,
        method: "POST",
        addr: "localhost:1337",
        apiEncoding: apiEncodingMock,
        rpc: rpcMock,
      });

      expect(result).toEqual(
        "curl 'http://localhost:1337/path?beta=1&beta=2&echo=true&key=some%20string' -H 'X-Alpha: some string' -H 'X-Delta: some string' -H 'X-Client-ID: some string' -d '{\"charile\":\"some string\"}'"
      );
    });

    it("should create curl for GET request", () => {
      const result = copyAsCurlToClipboard({
        serializeRequest,
        method: "GET",
        addr: "localhost:1337",
        apiEncoding: apiEncodingMock,
        rpc: rpcMock,
      });

      expect(result).toEqual(
        "curl 'http://localhost:1337/path?echo=true&key=some%20string' -H 'X-Alpha: some string' -H 'X-Client-ID: some string'"
      );
    });

    it("should supply default address if no address is given", () => {
      const result = copyAsCurlToClipboard({
        serializeRequest,
        method: "GET",
        addr: undefined,
        apiEncoding: apiEncodingMock,
        rpc: rpcMock,
      });

      expect(result?.includes("localhost:4000")).toEqual(true);
    });
  });
});
