-- encore.app --
{"id": ""}

-- package.json --
{"name": "ts-test-app"}

-- common-stuff/types.ts --
export interface ImportedRequest {
  name: string;
}

export interface ImportedResponse {
  message: string;
}

-- svc/encore.service.ts --
import { Service } from "encore.dev/service";

// Svc is a service for testing the client generator.
export default new Service("svc");

-- svc/svc.ts --
import { Header, Query, api, Gateway, Cookie } from "encore.dev/api";
import { authHandler } from "encore.dev/auth";
import type { ImportedRequest, ImportedResponse } from "../common-stuff/types";

interface UnusedType {
  foo: Foo;
}

// Root is a basic POST endpoint.
export const root = api(
  { expose: true, method: "POST", path: "/" },
  async (req: Request) => { },
);

// Imported tests the usage of imported types
// and this comment is also multiline.
export const imported = api(
  { expose: true, method: "POST", path: "/imported" },
  async (req: ImportedRequest) : Promise<ImportedResponse> => { },
);

export const onlyPathParams = api(
  { expose: true, method: "POST", path: "/path/:pathParam/:pathParam2" },
  async (req: { pathParam: string, pathParam2: string }) : Promise<ImportedResponse> => { },
);


export const dummy = api(
  { expose: true, method: "POST", path: "/dummy" },
  async (req: Request) => { },
);


export const noTypes = api(
  { expose: true, method: "POST", path: "/type-less" },
  async () => { },
)
export const cookiesOnly = api(
  { expose: true, method: "POST", path: "/cookies-only" },
  async (req: { field: Cookie<'cookie'> }): Promise<{ cookie: Cookie<'cookie'> }> => {
    return { cookie: { value: "value" } }
  },
)

export const cookieDummy = api(
  { expose: true, method: "POST", path: "/cookie-dummy" },
  async (req: Request): Promise<{ cookie: Cookie<'cookie'> }> => { return { cookie: { value: "value" } } },
);

export const singleSetCookie = api(
  { expose: true, method: "POST", path: "/single-set-cookie" },
  async (): Promise<{ message: string, token: Header<'set-cookie'> }> => { return { message: "ok", token: "session=abc" } },
);

export const multiSetCookie = api(
  { expose: true, method: "POST", path: "/multi-set-cookie" },
  async (): Promise<{ message: string, tokens: Header<string[], 'set-cookie'> }> => { return { message: "ok", tokens: ["a=1", "b=2"] } },
);

export interface AuthParams {
  cookie?: Header<'Cookie'>
  token?: Header<'x-api-token'>
  cookieValue?: Cookie<'actual-cookie'>
}

export interface AuthData {
  userID: string;
}

export const auth = authHandler<AuthParams, AuthData>(
  async (params) => {
    return { userID: "my-user-id" };
  }
)

export const gw = new Gateway({
  authHandler: auth,
})

// Request is the request type for testing doc comments on interfaces.
interface Request {
  // Foo is good
  foo?: number;
  // Baz is better
  baz: string;

  queryFoo?: Query<boolean, "foo">;
  queryBar?: Query<"bar">;
  queryList?: Query<boolean[], "list">
  headerBaz?: Header<"baz">;
  headerNum?: Header<number, "num">;
  cookieQux?: Cookie<"qux">;
  cookieQuux?: Cookie<number, "quux">;
}
