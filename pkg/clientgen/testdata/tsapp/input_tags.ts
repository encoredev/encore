-- encore.app --
{"id": ""}

-- package.json --
{"name": "ts-test-app"}

-- svc/encore.service.ts --
import { Service } from "encore.dev/service";

export default new Service("svc");

-- svc/svc.ts --
import { api } from "encore.dev/api";

interface Response {
  message: string;
}

// Ping has multiple tags, including a namespaced one.
export const ping = api(
  { expose: true, method: "GET", path: "/api/v1/entity/:id", tags: ["foo:bar", "example:shouldBeEmittedInOpenAPI"] },
  async (req: { id: string }): Promise<Response> => { },
);

// Pong shares a tag with ping.
export const pong = api(
  { expose: true, method: "POST", path: "/pong", tags: ["foo:bar"] },
  async (): Promise<Response> => { },
);

// Untagged has no tags at all.
export const untagged = api(
  { expose: true, method: "POST", path: "/untagged" },
  async (): Promise<Response> => { },
);
