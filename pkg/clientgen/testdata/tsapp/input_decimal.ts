-- encore.app --
{"id": ""}

-- package.json --
{"name": "ts-test-app"}

-- svc/svc.ts --
import { api } from "encore.dev/api";
import { Decimal } from "encore.dev/types";

export const dummy = api(
  { expose: true, method: "GET", path: "/dummy" },
  async (req: Request): Promise<Response> => {},
);

interface Request {
  message: string,
  val: Decimal,
}
interface Response {
  result: Decimal
}
