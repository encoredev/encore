import * as runtime from "../runtime/mod";
import { getCurrentRequest } from "../reqtrack/mod";
import { APIError, ErrCode } from "../../api/error";

export async function apiCall(
  service: string,
  endpoint: string,
  data: any,
): Promise<any> {
  const source = getCurrentRequest();
  const resp = await runtime.RT.apiCall(service, endpoint, data, source);

  // Convert any call error to our APIError type.
  // We do this here because NAPI doesn't have great support
  // for custom exception types yet.
  if (resp instanceof runtime.ApiCallError) {
    throw new APIError(resp.code as ErrCode, resp.message);
  }

  return resp;
}
