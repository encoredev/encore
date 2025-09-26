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

export async function middlewareDemo(params, opts) {
    const handler = (await import("../../../../myservice/api")).middlewareDemo;
    registerTestHandler({
        apiRoute: { service: "myservice", name: "middlewareDemo", raw: false, handler, streamingRequest: false, streamingResponse: false },
        middlewares: myservice_service.default.cfg.middlewares || [],
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
    });

    return apiCall("myservice", "middlewareDemo", params, opts);
}

export async function createUserViaService(params, opts) {
    const handler = (await import("../../../../myservice/api")).createUserViaService;
    registerTestHandler({
        apiRoute: { service: "myservice", name: "createUserViaService", raw: false, handler, streamingRequest: false, streamingResponse: false },
        middlewares: myservice_service.default.cfg.middlewares || [],
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
    });

    return apiCall("myservice", "createUserViaService", params, opts);
}

export async function getUsersViaService(params, opts) {
    const handler = (await import("../../../../myservice/api")).getUsersViaService;
    registerTestHandler({
        apiRoute: { service: "myservice", name: "getUsersViaService", raw: false, handler, streamingRequest: false, streamingResponse: false },
        middlewares: myservice_service.default.cfg.middlewares || [],
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
    });

    return apiCall("myservice", "getUsersViaService", params, opts);
}

export async function createItem(params, opts) {
    const handler = (await import("../../../../myservice/api")).createItem;
    registerTestHandler({
        apiRoute: { service: "myservice", name: "createItem", raw: false, handler, streamingRequest: false, streamingResponse: false },
        middlewares: myservice_service.default.cfg.middlewares || [],
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
    });

    return apiCall("myservice", "createItem", params, opts);
}

export async function getItem(params, opts) {
    const handler = (await import("../../../../myservice/api")).getItem;
    registerTestHandler({
        apiRoute: { service: "myservice", name: "getItem", raw: false, handler, streamingRequest: false, streamingResponse: false },
        middlewares: myservice_service.default.cfg.middlewares || [],
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
    });

    return apiCall("myservice", "getItem", params, opts);
}

