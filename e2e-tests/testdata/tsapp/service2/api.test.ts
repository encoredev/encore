import { describe, expect, it } from "vitest";
import { testApiError } from "./api";

describe("errors", () => {
  it("api error with no details with cause", async () => {
    await expect(
      testApiError({ variant: "no-details-with-cause" })
    ).rejects.toThrow(
      expect.objectContaining({
        message: "the error",
        details: undefined,
        code: "canceled",
        cause: expect.objectContaining({
          message: "this is the cause"
        })
      })
    );
  });

  it("api error with no details no cause", async () => {
    await expect(
      testApiError({ variant: "no-details-no-cause" })
    ).rejects.toThrow(
      expect.objectContaining({
        message: "the error",
        details: undefined,
        code: "canceled",
        cause: undefined
      })
    );
  });

  it("api error with details no cause", async () => {
    await expect(
      testApiError({ variant: "with-details-no-cause" })
    ).rejects.toThrow(
      expect.objectContaining({
        message: "the error",
        details: { a: "detail" },
        code: "canceled",
        cause: undefined
      })
    );
  });

  it("api error with details with cause", async () => {
    await expect(
      testApiError({ variant: "with-details-with-cause" })
    ).rejects.toThrow(
      expect.objectContaining({
        message: "the error",
        details: { a: "detail" },
        code: "canceled",
        cause: expect.objectContaining({
          message: "this is the cause"
        })
      })
    );
  });
});
