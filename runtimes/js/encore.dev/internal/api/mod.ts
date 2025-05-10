import * as runtime from "../runtime/mod";
import { getCurrentRequest } from "../reqtrack/mod";
import { APIError, ErrCode } from "../../api/error";

export async function apiCall(
  service: string,
  endpoint: string,
  data: any,
  opts?: runtime.CallOpts
): Promise<any> {
  const source = getCurrentRequest();
  const resp = await runtime.RT.apiCall(service, endpoint, data, source, opts);

  // Convert any call error to our APIError type.
  // We do this here because NAPI doesn't have great support
  // for custom exception types yet.
  if (resp instanceof runtime.ApiCallError) {
    throw new APIError(
      resp.code as ErrCode,
      resp.message,
      undefined,
      resp.details
    );
  }

  return resp;
}

export async function streamInOut(
  service: string,
  endpoint: string,
  data: any,
  opts?: runtime.CallOpts
): Promise<any> {
  const source = getCurrentRequest();
  const stream = await runtime.RT.stream(service, endpoint, data, source, opts);

  return {
    async send(msg: any) {
      stream.send(msg);
    },
    async recv(): Promise<any> {
      return stream.recv();
    },
    async close() {
      stream.close();
    },
    async *[Symbol.asyncIterator]() {
      while (true) {
        try {
          yield await stream.recv();
        } catch (e) {
          break;
        }
      }
    }
  };
}

export async function streamIn(
  service: string,
  endpoint: string,
  data: any,
  opts?: runtime.CallOpts
): Promise<any> {
  const source = getCurrentRequest();
  const stream = await runtime.RT.stream(service, endpoint, data, source, opts);
  const response = new Promise(async (resolve, reject) => {
    try {
      resolve(await stream.recv());
    } catch (e) {
      reject(e);
    }
  });

  return {
    async send(msg: any) {
      stream.send(msg);
    },
    async close() {
      stream.close();
    },
    async response(): Promise<any> {
      return response;
    }
  };
}

export async function streamOut(
  service: string,
  endpoint: string,
  data: any,
  opts?: runtime.CallOpts
): Promise<any> {
  const source = getCurrentRequest();
  const stream = await runtime.RT.stream(service, endpoint, data, source, opts);

  return {
    async recv(): Promise<any> {
      return stream.recv();
    },
    async close() {
      stream.close();
    },
    async *[Symbol.asyncIterator]() {
      while (true) {
        try {
          yield await stream.recv();
        } catch (e) {
          break;
        }
      }
    }
  };
}
