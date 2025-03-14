-- encore.app --
{"id": ""}

-- package.json --
{"name": "ts-test-app"}

-- svc/svc.ts --
import { Header, Query, api, Gateway } from "encore.dev/api";
import { authHandler } from "encore.dev/auth";

interface UnusedType {
  foo: Foo;
}

export const root = api(
  { expose: true, method: "POST", path: "/" },
  async (req: Request) => { },
);

export const dummy = api(
  { expose: true, method: "POST", path: "/dummy" },
  async (req: Request) => { },
);

export interface AuthParams {
  cookie?: Header<'Cookie'>
  token?: Header<'x-api-token'>
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
}
