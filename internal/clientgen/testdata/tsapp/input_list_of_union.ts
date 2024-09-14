-- encore.app --
{"id": ""}

-- package.json --
{"name": "ts-test-app"}

-- svc/svc.ts --
import { api } from "encore.dev/api";

export const dummy = api(
  { expose: true, method: "GET", path: "/dummy" },
  async (req: Request) => {},
);


interface Request {
    listOfUnion: ("a" | "b")[]
}

