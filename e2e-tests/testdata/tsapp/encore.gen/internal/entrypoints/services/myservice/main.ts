import { registerHandlers, run, type Handler } from "encore.dev/internal/codegen/appinit";
import { Worker, isMainThread } from "node:worker_threads";
import { fileURLToPath } from "node:url";
import { availableParallelism } from "node:os";

import { hello as helloImpl0 } from "../../../../../myservice/api";
import { middlewareDemo as middlewareDemoImpl1 } from "../../../../../myservice/api";
import { createUserViaService as createUserViaServiceImpl2 } from "../../../../../myservice/api";
import { getUsersViaService as getUsersViaServiceImpl3 } from "../../../../../myservice/api";
import { createItem as createItemImpl4 } from "../../../../../myservice/api";
import { getItem as getItemImpl5 } from "../../../../../myservice/api";
import * as myservice_service from "../../../../../myservice/encore.service";

const handlers: Handler[] = [
    {
        apiRoute: {
            service:           "myservice",
            name:              "hello",
            handler:           helloImpl0,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: myservice_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "myservice",
            name:              "middlewareDemo",
            handler:           middlewareDemoImpl1,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: myservice_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "myservice",
            name:              "createUserViaService",
            handler:           createUserViaServiceImpl2,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: myservice_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "myservice",
            name:              "getUsersViaService",
            handler:           getUsersViaServiceImpl3,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: myservice_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "myservice",
            name:              "createItem",
            handler:           createItemImpl4,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: myservice_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "myservice",
            name:              "getItem",
            handler:           getItemImpl5,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: myservice_service.default.cfg.middlewares || [],
    },
];

registerHandlers(handlers);

await run(import.meta.url);
