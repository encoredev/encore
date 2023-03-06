import JSONRPCConn from "./jsonrpc";
import { ResponseError } from "./errs";
import ReconnectingWebSocket from "reconnecting-websocket";

const DEV = import.meta.env.DEV;

export default class BaseClient {
  base: string;

  constructor() {
    this.base = DEV ? "localhost:9400" : window.location.host;
  }

  async do<T>(path: string, data?: any): Promise<T> {
    try {
      let resp = await this._do<T>(path, data);
      if (!resp.ok) {
        return Promise.reject(new ResponseError(path, resp.error.code, resp.error.detail, ""));
      }
      return Promise.resolve(resp.data);
    } catch (err) {
      return Promise.reject(new ResponseError(path, "network_error", null, err as any));
    }
  }

  ws(path: string): Promise<ReconnectingWebSocket> {
    const base = this.base;
    return new Promise<ReconnectingWebSocket>(function (resolve, reject) {
      let ws = new ReconnectingWebSocket(`ws://${base}${path}`);
      ws.addEventListener("open", () => {
        ws.onerror = null;
        resolve(ws);
      });

      ws.addEventListener("error", (err: any) => {
        if (DEV) {
          reject(
            new Error(
              "could not connect to Encore daemon in development mode. to start it, run: ENCORE_DAEMON_DEV=1 encore daemon -f"
            )
          );
        }
        reject(new ResponseError(path, "network", null, err));
      });
    });
  }

  async jsonrpc(path: string): Promise<JSONRPCConn> {
    const ws = await this.ws(path);
    return new JSONRPCConn(ws);
  }

  async _do<T>(path: string, data?: any): Promise<APIResponse<T>> {
    let body = null;
    if (data) {
      body = JSON.stringify(data);
    }

    let resp = await fetch(`http://${this.base}${path}`, {
      method: "POST",
      body: body,
    });
    return await resp.json();
  }
}

interface ErrorResponse {
  ok: false;
  error: {
    code: string;
    detail: any;
  };
}

interface SuccessResponse<T> {
  ok: true;
  data: T;
}

type APIResponse<T> = SuccessResponse<T> | ErrorResponse;
