import { api, HttpStatus } from "encore.dev/api";
import { IsEmail, MaxLen, MinLen } from "encore.dev/validate";
import log from "encore.dev/log";
import { APIError } from "encore.dev/api";

interface GreetingRequest {
  name: string;
  style: "formal" | "casual" | "excited";
}

interface GreetingResponse {
  greeting: string;
  timestamp: Date;
}

interface StatusResponse {
  status: "healthy" | "busy" | "maintenance";
  uptime: string;
  version: string;
}

interface MessageRequest {
  message: string & MinLen<3> & MaxLen<1000>;
  priority?: "low" | "medium" | "high";
  recipient?: string & IsEmail;
}

interface MessageResponse {
  message: string;
  processed: boolean;
  status: HttpStatus;
}

// Generate different greeting styles
export const greet = api(
  { expose: true, method: "POST", path: "/greet" },
  async (req: GreetingRequest): Promise<GreetingResponse> => {
    log.info("Generating greeting", { name: req.name, style: req.style });

    let greeting: string;

    switch (req.style) {
      case "formal":
        greeting = `Good day, ${req.name}. I trust you are well.`;
        break;
      case "casual":
        greeting = `Hey ${req.name}! How's it going?`;
        break;
      case "excited":
        greeting = `OMG ${req.name}!!! So great to see you!!!`;
        break;
      default:
        greeting = `Hello, ${req.name}.`;
    }

    return {
      greeting,
      timestamp: new Date()
    };
  }
);

// Process a message with validation and different responses
export const processMessage = api(
  { expose: true, method: "POST", path: "/process-message" },
  async (req: MessageRequest): Promise<MessageResponse> => {
    log.info("Processing message", {
      messageLength: req.message.length,
      priority: req.priority,
      hasRecipient: !!req.recipient
    });

    // Simulate different processing outcomes
    const priority = req.priority || "medium";
    let processed = true;
    let status: HttpStatus = HttpStatus.OK;

    if (req.message.includes("error")) {
      processed = false;
      status = HttpStatus.BadRequest;
    } else if (req.message.includes("urgent")) {
      status = HttpStatus.Accepted;
    } else {
      status = HttpStatus.Created;
    }

    return {
      message: `Message processed with ${priority} priority`,
      processed,
      status
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
