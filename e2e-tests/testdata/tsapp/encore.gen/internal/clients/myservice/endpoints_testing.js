import { apiCall, streamIn, streamOut, streamInOut } from "encore.dev/internal/codegen/api";
import { registerTestHandler } from "encore.dev/internal/codegen/appinit";

import * as myservice_service from "../../../../myservice/encore.service";

export async function hello(params, opts) {
    const handler = (await import("../../../../myservice/api")).hello;
    registerTestHandler({
        apiRoute: { service: "myservice", name: "hello", raw: false, handler, streamingRequest: false, streamingResponse: false },
        middlewares: myservice_service.default.cfg.middlewares || [],
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
    });

    return apiCall("myservice", "hello", params, opts);
}

