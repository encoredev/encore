import { apiCall, streamIn, streamOut, streamInOut } from "encore.dev/internal/codegen/api";

const TEST_ENDPOINTS = typeof ENCORE_DROP_TESTS === "undefined" && process.env.NODE_ENV === "test"
    ? await import("./endpoints_testing.js")
    : null;

export async function hello(params, opts) {
    if (typeof ENCORE_DROP_TESTS === "undefined" && process.env.NODE_ENV === "test") {
        return TEST_ENDPOINTS.hello(params, opts);
    }

    return apiCall("myservice", "hello", params, opts);
}
export async function middlewareDemo(opts) {
    const params = undefined;
    if (typeof ENCORE_DROP_TESTS === "undefined" && process.env.NODE_ENV === "test") {
        return TEST_ENDPOINTS.middlewareDemo(params, opts);
    }

    return apiCall("myservice", "middlewareDemo", params, opts);
}
export async function createUserViaService(params, opts) {
    if (typeof ENCORE_DROP_TESTS === "undefined" && process.env.NODE_ENV === "test") {
        return TEST_ENDPOINTS.createUserViaService(params, opts);
    }

    return apiCall("myservice", "createUserViaService", params, opts);
}
export async function getUsersViaService(opts) {
    const params = undefined;
    if (typeof ENCORE_DROP_TESTS === "undefined" && process.env.NODE_ENV === "test") {
        return TEST_ENDPOINTS.getUsersViaService(params, opts);
    }

    return apiCall("myservice", "getUsersViaService", params, opts);
}
export async function createItem(params, opts) {
    if (typeof ENCORE_DROP_TESTS === "undefined" && process.env.NODE_ENV === "test") {
        return TEST_ENDPOINTS.createItem(params, opts);
    }

    return apiCall("myservice", "createItem", params, opts);
}
export async function getItem(params, opts) {
    if (typeof ENCORE_DROP_TESTS === "undefined" && process.env.NODE_ENV === "test") {
        return TEST_ENDPOINTS.getItem(params, opts);
    }

    return apiCall("myservice", "getItem", params, opts);
}
