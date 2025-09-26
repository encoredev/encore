import { api, HttpStatus } from "encore.dev/api";
import { APICallMeta, currentRequest } from "encore.dev";
import { service2 } from "~encore/clients";
import log from "encore.dev/log";

export const hello = api(
  { expose: true, method: "GET", path: "/hello/:name" },
  async ({ name }: { name: string }): Promise<{ message: string }> => {
    return { message: `Hello ${name}` };
  }
);

// Endpoint that demonstrates middleware data access
export const middlewareDemo = api(
  { expose: true, method: "GET", path: "/middleware-test", tags: ["mwtest"] },
  async (): Promise<{
    message: string;
    middlewareMsg: string;
  }> => {
    const req = currentRequest() as APICallMeta;

    return {
      message: "Hello",
      middlewareMsg: req.middlewareData?.customMsg || "Not set"
    };
  }
);

// Service-to-service call: Get greeting from service2
export const getGreetingViaService2 = api(
  { expose: true, method: "POST", path: "/get-greeting" },
  async (req: {
    name: string;
    style?: "formal" | "casual" | "excited";
  }): Promise<{
    message: string;
    greeting: string;
  }> => {
    try {
      const result = await service2.greet({
        name: req.name,
        style: req.style || "formal"
      });

      return {
        message: "Greeting retrieved successfully via service-to-service call",
        greeting: result.greeting
      };
    } catch (error) {
      log.error("Failed to get greeting via service call", { error });
      throw new Error(
        "Service-to-service call failed: " + (error as Error).message
      );
    }
  }
);

// Endpoint with custom HTTP status
export const customStatus = api(
  { expose: true, method: "GET", path: "/test-custom-status" },
  async (): Promise<{
    message: string;
    status: HttpStatus;
  }> => {
    return {
      message: "I accept!",
      status: 202
    };
  }
);
