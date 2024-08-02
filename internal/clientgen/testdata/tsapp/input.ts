-- encore.app --
{"id": ""}

-- package.json --
{"name": "ts-test-app"}

-- svc/svc.ts --
import { Header, Query, api } from "encore.dev/api";

interface UnusedType {
  foo: Foo;
}

export const dummy = api(
  { expose: true, method: "GET", path: "/dummy" },
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

