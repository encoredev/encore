import { api, HttpStatus } from "encore.dev/api";
import { IsEmail, MaxLen, MinLen } from "encore.dev/validate";
import log from "encore.dev/log";
import { APIError } from "encore.dev/api";

interface GreetingRequest {
  name: string;
}

interface GreetingResponse {
  greeting: string;
  timestamp: Date;
}

interface MessageRequest {
  message: string & MinLen<3> & MaxLen<1000>;
  recipient?: string & IsEmail;
}

interface MessageResponse {
  message: string;
}

// Generate different greeting styles
export const greet = api(
  { expose: true, method: "POST", path: "/greet" },
  async (req: GreetingRequest): Promise<GreetingResponse> => {
    let greeting = `Hey ${req.name}! How's it going?`;
    return {
      greeting,
      timestamp: new Date()
    };
  }
);

export const testInputValidation = api(
  { expose: true, method: "POST", path: "/test-validation" },
  async (req: MessageRequest): Promise<MessageResponse> => {
    return {
      message: `Message processed`
    };
  }
);

export const testApiError = api(
  { expose: true, method: "GET", path: "/test-api-error/:variant" },
  async ({ variant }: { variant: string }): Promise<{ message: string }> => {
    switch (variant) {
      case "no-details-no-cause":
        throw APIError.canceled("the error");
      case "with-details-no-cause":
        throw APIError.canceled("the error").withDetails({ a: "detail" });
      case "no-details-with-cause":
        throw APIError.canceled("the error", new Error("this is the cause"));
      case "with-details-with-cause":
        throw APIError.canceled(
          "the error",
          new Error("this is the cause")
        ).withDetails({ a: "detail" });
      default:
        return { message: "Hello there" };
    }
  }
);

export const testOtherError = api(
  { expose: true, method: "GET", path: "/test-other-error" },
  async (): Promise<{ message: string }> => {
    throw new Error("This is a test error");
  }
);
