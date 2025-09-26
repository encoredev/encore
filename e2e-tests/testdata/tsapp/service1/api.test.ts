import { describe, expect, it } from "vitest";
import { hello } from "./api";

describe("get", () => {
  it("should combine string with parameter value", async () => {
    const resp = await hello({ name: "world" });
    expect(resp.message).toBe("Hello world");
  });
});
