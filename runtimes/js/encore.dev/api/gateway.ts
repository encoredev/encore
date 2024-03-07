import { AuthHandlerBrand } from "../auth/mod";
import * as runtime from "../internal/runtime/mod";
import { setCurrentRequest } from "../internal/reqtrack/mod";

export class Gateway {
  public readonly name: string;
  public readonly cfg: GatewayConfig;
  private impl: runtime.Gateway;

  constructor(cfg: GatewayConfig) {
    this.name = "api-gateway";
    this.cfg = cfg;

    let auth: any = cfg.authHandler;
    if (auth) {
      const handler = auth;
      auth = (req: runtime.Request) => {
        setCurrentRequest(req);
        return handler(req.payload());
      };
    }

    this.impl = runtime.RT.gateway("api-gateway", {
      auth,
    });
  }
}

export interface GatewayConfig {
  authHandler?: AuthHandlerBrand;
}
