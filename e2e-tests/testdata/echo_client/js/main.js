import "isomorphic-fetch";
import { deepEqual } from "assert";

import Client, { ErrCode, isAPIError } from "./client.js";

if (process.argv.length < 3) {
  console.error("Usage: npm run test -- <host:port>");
  console.log(`Got ${process.argv.length} arguments`);
  process.exit(1);
}

// Create the client
const api = new Client("http://" + process.argv[2]);

// Test a simple no-op
await api.test.Noop();

// Test we get back the right structured error
await assertStructuredError(
  api.test.NoopWithError(),
  ErrCode.Unimplemented,
  "totally not implemented yet"
);

// Test a simple echo
const echoRsp = await api.test.SimpleBodyEcho({ Message: "hello world" });
deepEqual(echoRsp.Message, "hello world", "Wanted body to be 'hello world'");

// Check our UpdateMessage and GetMessage API's
let getRsp = await api.test.GetMessage("javascript");
deepEqual(getRsp.Message, "", "Expected no message on first request");

await api.test.UpdateMessage("javascript", { Message: "updating now" });

getRsp = await api.test.GetMessage("javascript");
deepEqual(getRsp.Message, "updating now", "Expected data from Update request");

// Test the rest API which uses all input types (query string, json body and header fields)
// as well as nested structs and path segments in the URL
const restRsp = await api.test.RestStyleAPI(5, "hello", {
  HeaderValue: "this is the header field",
  QueryValue: "this is a query string field",
  "Some-Key": "this is the body field",
  Nested: {
    Alice: "the nested key",
    bOb: 8,
    charile: true
  }
});
deepEqual(
  restRsp.HeaderValue,
  "this is the header field",
  "expected header value"
);
deepEqual(
  restRsp.QueryValue,
  "this is a query string field",
  "expected query value"
);
deepEqual(restRsp["Some-Key"], "this is the body field", "expected body value");
deepEqual(
  restRsp.Nested.Alice,
  "hello + the nested key",
  "expected nested key"
);
deepEqual(restRsp.Nested.bOb, 5 + 8, "expected nested value");
deepEqual(restRsp.Nested.charile, true, "expected nested ok");

// Full marshalling test with randomised payloads
function rInt() {
  return Math.floor(Math.random() * 10000000);
}

const params = {
  HeaderBoolean: Math.random() > 0.5,
  HeaderInt: rInt(),
  HeaderFloat: Math.random(),
  HeaderString: "header string",
  HeaderBytes: "aGVsbG8K",
  HeaderTime: new Date(Math.floor(Date.now() / 1000) * 1000)
    .toISOString()
    .replace(".000Z", "Z"),
  HeaderJson: { hello: "world" },
  HeaderUUID: "2553e3a4-5d9f-4716-82a2-b9bdc20a3263",
  HeaderUserID: "432",
  HeaderOption: "test",
  QueryBoolean: Math.random() > 0.5,
  QueryInt: rInt(),
  QueryFloat: Math.random(),
  QueryString: "query string",
  QueryBytes: "d29ybGQK",
  QueryTime: new Date(Math.floor(Date.now() / 1000) * 1000)
    .toISOString()
    .replace(".000Z", "Z"),
  QueryJson: { value: true },
  QueryUUID: "84b7463d-6000-4678-9d94-1d526bb5217c",
  QueryUserID: "9udfa",
  QuerySlice: [rInt(), rInt(), rInt(), rInt()],
  boolean: Math.random() > 0.5,
  int: Math.floor(Math.random() * 10000000),
  float: Math.random(),
  string: "body string",
  bytes: "aXMgaXQgbWUgeW91IGFyZSBsb29raW5nIGZvcj8K",
  time: new Date(Math.floor(Date.now() / 1000) * 1000)
    .toISOString()
    .replace(".000Z", "Z"),
  json: { json_value: 4321 },
  uuid: "c227acf4-1902-4c85-8027-623d47ef4c8a",
  "user-id": "✉️",
  slice: [rInt(), rInt(), rInt(), rInt(), rInt(), rInt()],
  option: 5,
  "option-slice": [1, null, 2]
};
const mResp = await api.test.MarshallerTestHandler(params);
deepEqual(mResp, params, "Expected the same response from the marshaller test");

// Test auth handlers
await assertStructuredError(
  api.test.TestAuthHandler(),
  ErrCode.Unauthenticated,
  "missing auth param"
);

// Test with static auth data
{
  const api = new Client("http://" + process.argv[2], {
    auth: {
      AuthInt: 34,
      Authorization: "Bearer tokendata",
      NewAuth: false,
      Header: "",
      Query: []
    }
  });

  const resp = await api.test.TestAuthHandler();
  deepEqual(resp.Message, "user::true", "expected the user ID back");
}

// Test with auth data generator function
{
  let tokenToReturn = "tokendata";
  const api = new Client("http://" + process.argv[2], {
    auth: () => {
      return {
        Authorization: "Bearer " + tokenToReturn,
        AuthInt: 34,
        NewAuth: false,
        Header: "",
        Query: []
      };
    }
  });

  // With a valid token
  const resp = await api.test.TestAuthHandler();
  deepEqual(resp.Message, "user::true", "expected the user ID back");

  // With an invalid token
  tokenToReturn = "invalid-token-value";
  await assertStructuredError(
    api.test.TestAuthHandler(),
    ErrCode.Unauthenticated,
    "invalid token"
  );
}

// Test with headers and query string auth data
{
  const api = new Client("http://" + process.argv[2], {
    auth: {
      Authorization: "",
      AuthInt: 34,
      NewAuth: true,
      Header: "102",
      Query: [42, 100, -50, 10]
    }
  });

  const resp = await api.test.TestAuthHandler();
  deepEqual(resp.Message, "second_user::true", "expected the user ID back");
}

// Test the raw endpoint
{
  const api = new Client("http://" + process.argv[2], {
    auth: {
      AuthInt: 34,
      Authorization: "Bearer tokendata",
      NewAuth: false,
      Header: "",
      Query: []
    }
  });

  const resp = await api.test.RawEndpoint(
    "PUT",
    ["hello"],
    "this is a test body",
    {
      headers: { "X-Test-Header": "test" },
      query: { foo: "bar" }
    }
  );

  deepEqual(resp.status, 201, "expected the status code to be 201");

  const response = await resp.json();

  deepEqual(
    response,
    {
      Body: "this is a test body",
      Header: "test",
      PathParam: "hello",
      QueryString: "bar"
    },
    "expected the response to match"
  );
}

// Test path encoding
const resp = await api.test.PathMultiSegments(
  true,
  342,
  "foo/blah/should/get/escaped",
  "503f4487-1e15-4c37-9a80-7b70f86387bb",
  ["foo/bar", "blah", "seperate/segments = great success"]
);
deepEqual(resp.Boolean, true, "expected the boolean to be true");
deepEqual(resp.Int, 342, "expected the int to be 342");
deepEqual(
  resp.String,
  "foo/blah/should/get/escaped",
  "invalid string field returned"
);
deepEqual(
  resp.UUID,
  "503f4487-1e15-4c37-9a80-7b70f86387bb",
  "invalid UUID returned"
);
deepEqual(
  resp.Wildcard,
  "foo/bar/blah/seperate/segments = great success",
  "invalid wildcard field returned"
);

// Test validation
{
  await api.validation.TestOne({
    Msg: "pass"
  });
  await assertStructuredError(
    api.validation.TestOne({ Msg: "fail" }),
    ErrCode.InvalidArgument,
    "validation failed: bad message"
  );
  const client = new Client("http://" + process.argv[2], {
    auth: {
      AuthInt: 0,
      Authorization: "",
      NewAuth: false,
      Header: "fail-validation",
      Query: []
    }
  });
  await assertStructuredError(
    client.test.Noop(),
    ErrCode.InvalidArgument,
    "validation failed: auth validation fail"
  );
}

// Test middleware
{
  await assertStructuredError(
    api.middleware.Error(),
    ErrCode.Internal,
    "middleware error"
  );
  const resp1 = await api.middleware.ResponseRewrite({ Msg: "foo" });
  deepEqual(resp1.Msg, "middleware(req=foo, resp=handler(foo))");

  const resp2 = await api.middleware.ResponseGen({ Msg: "foo" });
  deepEqual(resp2.Msg, "middleware generated");
}

// Client test completed
process.exit(0);

async function assertStructuredError(promise, code, message) {
  let errorOccurred = false;
  try {
    await promise;
  } catch (err) {
    errorOccurred = true;
    if (isAPIError(err)) {
      if (err.code !== code) {
        throw new Error(
          `Expected error code ${code}, got ${err.code} with message "${err.message}"`
        );
      }
      if (err.message !== message) {
        throw new Error(
          `Expected error message "${message}", got "${err.message}"`
        );
      }
    } else {
      throw new Error(`Expected APIError, got ${err}`);
    }
  }

  if (!errorOccurred) {
    throw new Error("No error was thrown during call to NoopWithError");
  }
}
