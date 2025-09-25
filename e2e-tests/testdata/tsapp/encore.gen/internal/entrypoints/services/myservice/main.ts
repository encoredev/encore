import { registerHandlers, run, type Handler } from "encore.dev/internal/codegen/appinit";
import { Worker, isMainThread } from "node:worker_threads";
import { fileURLToPath } from "node:url";
import { availableParallelism } from "node:os";

import { hello as helloImpl0 } from "../../../../../myservice/api";
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
];

registerHandlers(handlers);

await run(import.meta.url);
