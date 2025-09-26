import { registerGateways, registerHandlers, run, type Handler } from "encore.dev/internal/codegen/appinit";

import { hello as service1_helloImpl0 } from "../../../../service1/api";
import { middlewareDemo as service1_middlewareDemoImpl1 } from "../../../../service1/api";
import { getGreetingViaService2 as service1_getGreetingViaService2Impl2 } from "../../../../service1/api";
import { customStatus as service1_customStatusImpl3 } from "../../../../service1/api";
import { greet as service2_greetImpl4 } from "../../../../service2/api";
import { processMessage as service2_processMessageImpl5 } from "../../../../service2/api";
import { testApiError as service2_testApiErrorImpl6 } from "../../../../service2/api";
import { testOtherError as service2_testOtherErrorImpl7 } from "../../../../service2/api";
import * as service1_service from "../../../../service1/encore.service";
import * as service2_service from "../../../../service2/encore.service";

const gateways: any[] = [
];

const handlers: Handler[] = [
    {
        apiRoute: {
            service:           "service1",
            name:              "hello",
            handler:           service1_helloImpl0,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: service1_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "service1",
            name:              "middlewareDemo",
            handler:           service1_middlewareDemoImpl1,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":["mwtest"]},
        middlewares: service1_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "service1",
            name:              "getGreetingViaService2",
            handler:           service1_getGreetingViaService2Impl2,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: service1_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "service1",
            name:              "customStatus",
            handler:           service1_customStatusImpl3,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: service1_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "service2",
            name:              "greet",
            handler:           service2_greetImpl4,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: service2_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "service2",
            name:              "processMessage",
            handler:           service2_processMessageImpl5,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: service2_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "service2",
            name:              "testApiError",
            handler:           service2_testApiErrorImpl6,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: service2_service.default.cfg.middlewares || [],
    },
    {
        apiRoute: {
            service:           "service2",
            name:              "testOtherError",
            handler:           service2_testOtherErrorImpl7,
            raw:               false,
            streamingRequest:  false,
            streamingResponse: false,
        },
        endpointOptions: {"expose":true,"auth":false,"isRaw":false,"isStream":false,"tags":[]},
        middlewares: service2_service.default.cfg.middlewares || [],
    },
];

registerGateways(gateways);
registerHandlers(handlers);

await run(import.meta.url);
