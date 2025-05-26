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

-- svc/svc.ts --
import { Header, Query, api, Gateway, Cookie } from "encore.dev/api";
import { authHandler } from "encore.dev/auth";
import type { ImportedRequest, ImportedResponse } from "../common-stuff/types";

interface UnusedType {
  foo: Foo;
}

export const root = api(
  { expose: true, method: "POST", path: "/" },
  async (req: Request) => { },
);

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

interface Request {
  // Foo is good
  foo?: number;
  // Baz is better
  baz: string;

  queryFoo?: Query<boolean, "foo">;
  queryBar?: Query<"bar">;
  headerBaz?: Header<"baz">;
  headerNum?: Header<number, "num">;
  cookieQux?: Cookie<"qux">;
  cookieQuux?: Cookie<number, "quux">;
}
