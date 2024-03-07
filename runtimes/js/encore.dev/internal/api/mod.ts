import * as runtime from "../runtime/mod";
import { getCurrentRequest } from "../reqtrack/mod";

export async function apiCall(
  service: string,
  endpoint: string,
  data: any,
): Promise<any> {
  const source = getCurrentRequest();
  return runtime.RT.apiCall(service, endpoint, data, source);
}