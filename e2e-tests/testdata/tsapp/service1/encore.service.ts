import { Service } from "encore.dev/service";
import { middleware } from "encore.dev/api";

// Middleware to add custom data to request
const dataEnricher = middleware(async (req, next) => {
  // Add custom data that endpoints can access
  req.data.customMsg = "Hello from middleware!";

  const resp = await next(req);

  resp.status = 201;
  resp.header.add("x-test-header", "hello");

  return resp;
});

export default new Service("service1", {
  middlewares: [middleware({ target: { tags: ["mwtest"] } }, dataEnricher)]
});
