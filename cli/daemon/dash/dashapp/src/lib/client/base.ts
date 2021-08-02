import JSONRPCConn from "./jsonrpc";
import { ResponseError } from "./errs";

export default class BaseClient {
  base: string;

  constructor() {
    this.base = import.meta.env.VITE_DAEMON_ADDRESS ?? window.location.host
  }

  async do<T>(path: string, data?: any): Promise<T> {
    try {
      let resp = await this._do<T>(path, data);
      if (!resp.ok) {
        return Promise.reject(new ResponseError(path, resp.error.code, resp.error.detail, ""));
      }
      return Promise.resolve(resp.data);
    } catch (err) {
      return Promise.reject(new ResponseError(path, "network_error", null, err))
    }
  }

  ws(path: string): Promise<WebSocket> {
    const base = this.base
    return new Promise<WebSocket>(function(resolve, reject) {
      let ws = new WebSocket(`ws://${base}${path}`);
      ws.onopen = function() {
        ws.onerror = null
        resolve(ws)
      };
      ws.onerror = function(err: any) {
        reject(new ResponseError(path, "network", null, err))
      }
    })
  }

  async jsonrpc(path: string): Promise<JSONRPCConn> {
    const ws = await this.ws(path)
    return new JSONRPCConn(ws)
  }

  async _do<T>(path: string, data?: any): Promise<APIResponse<T>> {
    let body = null
    if (data) {
      body = JSON.stringify(data)
    }

    let resp = await fetch(`http://${this.base}${path}`, {
      method: "POST",
      body: body,
    })
    return await resp.json()
  }
}

interface ErrorResponse {
  ok: false;
  error: {
    code: string;
    detail: any;
  }
}

interface SuccessResponse<T> {
  ok: true;
  data: T;
}

type APIResponse<T> = SuccessResponse<T> | ErrorResponse;