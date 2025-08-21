-- encore.app --
{"id": ""}

-- package.json --
{"name": "ts-test-app"}

-- svc/svc.ts --
import { api, HttpStatus } from "encore.dev/api";

export const dummy = api(
  { expose: true, method: "GET", path: "/dummy" },
  async (): Promise<Response> => {},
);


interface Response {
  message: string,
  status: HttpStatus,
}
