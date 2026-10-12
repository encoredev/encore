import { expect, it, vi } from "vitest";
import { service1, service2 } from "~encore/clients";

const loaded = vi.hoisted(() => ({
  service1: vi.fn(),
  service2: vi.fn()
}));

vi.mock("./service1/encore.service", async (importOriginal) => {
  loaded.service1();
  return importOriginal();
});
vi.mock("./service2/encore.service", async (importOriginal) => {
  loaded.service2();
  return importOriginal();
});

// Keep the lifecycle in one test: the initial assertions must run before any
// client call, and the final reset must exercise the original client exports.
it("loads service test modules on demand and respects module resets", async () => {
  expect(loaded.service1).not.toHaveBeenCalled();
  expect(loaded.service2).not.toHaveBeenCalled();
  expect(service2.ref()).toBe(service2.ref());
  expect(loaded.service2).not.toHaveBeenCalled();

  // Concurrent first calls share module initialization. Importing api.ts also
  // imports the client catalog again, exercising the circular import path.
  const [alice, bob, middleware] = await Promise.all([
    service1.hello({ name: "Alice" }),
    service1.ref().hello({ name: "Bob" }),
    service1.middlewareDemo()
  ]);
  expect(alice).toEqual({ message: "Hello Alice" });
  expect(bob).toEqual({ message: "Hello Bob" });
  expect(middleware).toEqual({
    message: "Hello",
    middlewareMsg: "Hello from middleware!"
  });
  expect(loaded.service1).toHaveBeenCalledTimes(1);
  expect(loaded.service2).not.toHaveBeenCalled();

  const greeting = await service1.getGreetingViaService2({ name: "Charlie" });
  expect(greeting.greeting).toContain("Charlie");
  expect(loaded.service2).toHaveBeenCalledTimes(1);

  // Handler lookup and registration must continue to happen on every call.
  const hello = vi.fn(async () => ({ message: "mocked" }));
  vi.doMock("./service1/api", () => ({ hello }));
  try {
    expect(await service1.hello({ name: "ignored" })).toEqual({ message: "mocked" });
    expect(hello).toHaveBeenCalledTimes(1);
  } finally {
    vi.doUnmock("./service1/api");
  }

  // A promise retained by the original client wrapper would bypass this new
  // service module and keep using the old test module after resetModules().
  vi.resetModules();
  vi.doMock("./service1/encore.service", () => {
    throw new Error("service initialization failed");
  });
  try {
    // Vitest wraps errors from mock factories; retain the initialization cause.
    await expect(service1.hello({ name: "Alice" })).rejects.toMatchObject({
      cause: expect.objectContaining({ message: "service initialization failed" })
    });
  } finally {
    vi.doUnmock("./service1/encore.service");
  }
});
