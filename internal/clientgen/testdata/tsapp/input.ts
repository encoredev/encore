-- encore.app --
{"id": ""}

-- package.json --
{"name": "ts-test-app"}

-- svc/svc.ts --
import { Header, Query, api } from "encore.dev/api";

interface UnusedType {
  foo: Foo;
}

export const dummy1 = api(
  { expose: true, method: "POST", path: "/dummy" },
  async (req: Request) => {},
);
export const dummy2 = api(
  { expose: true, method: "POST", path: "/other_dummy" },
  async (req: Request) => {},
);
export const dummy5 = api(
  { expose: true, method: "POST", path: "/dummy/:foo" },
  async (req: Request) => {},
);


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

