// Code generated by the Encore v0.0.0-develop client generator. DO NOT EDIT.

// Disable eslint, jshint, and jslint for this file.
/* eslint-disable */
/* jshint ignore:start */
/*jslint-disable*/

/**
 * Local is the base URL for calling the Encore application's API.
 */
export const Local = "http://localhost:4000"

/**
 * Environment returns a BaseURL for calling the cloud environment with the given name.
 */
export function Environment(name) {
    return `https://${name}-app.encr.app`
}

/**
 * PreviewEnv returns a BaseURL for calling the preview environment with the given PR number.
 */
export function PreviewEnv(pr) {
    return Environment(`pr${pr}`)
}

/**
 * Client is an API client for the app Encore application.
 */
export default class Client {
    /**
     * Creates a Client for calling the public and authenticated APIs of your Encore application.
     *
     * @param target  The target which the client should be configured to use. See Local and Environment for options.
     * @param options Options for the client
     */
    constructor(target = "prod", options = undefined) {
        // Convert the old constructor parameters to a BaseURL object and a ClientOptions object
        if (!target.startsWith("http://") && !target.startsWith("https://")) {
            target = Environment(target)
        }

        if (typeof options === "string") {
            options = { auth: options }
        }

        const base = new BaseClient(target, options ?? {})
        this.svc = new svc.ServiceClient(base)
    }
}

class SvcServiceClient {
    constructor(baseClient) {
        this.baseClient = baseClient
    }

    /**
     * DummyAPI is a dummy endpoint.
     */
    async DummyAPI(params) {
        await this.baseClient.callAPI("POST", `/svc.DummyAPI`, JSON.stringify(params))
    }

    /**
     * Private is a basic auth endpoint.
     */
    async Private(params) {
        await this.baseClient.callAPI("POST", `/svc.Private`, JSON.stringify(params))
    }
}

export const svc = {
    ServiceClient: SvcServiceClient
}


function encodeQuery(parts) {
    const pairs = []
    for (const key in parts) {
        const val = (Array.isArray(parts[key]) ?  parts[key] : [parts[key]])
        for (const v of val) {
            pairs.push(`${key}=${encodeURIComponent(v)}`)
        }
    }
    return pairs.join("&")
}

// makeRecord takes a record and strips any undefined values from it,
// and returns the same record with a narrower type.
function makeRecord(record) {
    for (const key in record) {
        if (record[key] === undefined) {
            delete record[key]
        }
    }
    return record
}


function encodeWebSocketHeaders(headers) {
    // url safe, no pad
    const base64encoded = btoa(JSON.stringify(headers))
      .replaceAll("=", "")
      .replaceAll("+", "-")
      .replaceAll("/", "_");
    return "encore.dev.headers." + base64encoded;
}

class WebSocketConnection {
    done = false;
    retry = 0;
    minDelayMs = 10;
    maxDelayMs = 500;

    hasUpdateHandlers = [];
    msgHandler = undefined;
    errorHandler = undefined;

    constructor(url, headers) {
        let protocols = ["encore-ws"];
        if (headers) {
            protocols.push(encodeWebSocketHeaders(headers));
        }

        this.protocols = protocols;
        this.url = url;

        this.ws = this.connect();
    }

    connect() {
        const ws = new WebSocket(this.url, this.protocols);

        ws.addEventListener("open", (_event) => {
            this.retry = 0;
        });

        ws.addEventListener("error", (event) => {
            if (this.errorHandler !== undefined) {
                this.errorHandler(event);
            }
            this.ws.close(1002);
        });

        ws.addEventListener("message", (event) => {
            if (this.msgHandler !== undefined) {
                this.msgHandler(event);
            }
            this.resolveHasUpdateHandlers();
        });

        ws.addEventListener("close", (event) => {
            // normal closure, no reconnect
            if (event.code === 1005 || event.code === 1000) {
                this.done = true;
            }
            if (!this.done) {
                const delay = Math.min(this.minDelayMs * 2 ** this.retry, this.maxDelayMs);
                console.log(`Reconnecting to ${this.url} in ${delay}ms`);
                this.retry += 1;
                setTimeout(() => {
                    this.ws = this.connect();
                }, delay);
            }
            this.resolveHasUpdateHandlers();
        });

        return ws
    }

    resolveHasUpdateHandlers() {
        const handlers = this.hasUpdateHandlers;
        this.hasUpdateHandlers = [];

        for (const handler of handlers) {
            handler()
        }
    }

    async hasUpdate() {
        // await until a new message have been received, or the socket is closed
        await new Promise((resolve) => {
            this.hasUpdateHandlers.push(() => resolve(null))
        });
    }

    setMsgHandler(handler) {
        this.msgHandler = handler;
    }

    setErrorHandler(handler) {
        this.errorHandler = handler;
    }

    close() {
        this.done = true;
        this.ws.close();
    }
}

export class BidiStream {
    buffer = [];

    constructor(url, headers) {
        this.connection = new WebSocketConnection(url, headers);
        this.connection.setMsgHandler((event) => {
            this.buffer.push(JSON.parse(event.data));
        });
    }

    close() {
        this.connection.close();
    }

    on(type, handler) {
        if (type === "error")
            this.connection.setErrorHandler(handler);
    }

    async send(msg) {
        if (this.connection.ws.readyState === WebSocket.CONNECTING) {
            // await that the socket is opened
            await new Promise((resolve) => {
                const handler = () => {
                    this.connection.ws.removeEventListener("open", handler);
                    resolve();
                };
                this.connection.ws.addEventListener("open", handler);
            });
        }

        return this.connection.ws.send(JSON.stringify(msg));
    }

    async next() {
        for await (const next of this) return next;
    }

    async *[Symbol.asyncIterator]() {
        while (true) {
            if (this.buffer.length > 0) {
                yield this.buffer.shift();
            } else {
                if (this.connection.done) break;
                await this.connection.hasUpdate();
            }
        }
    }
}

export class InStream {
    buffer = [];

    constructor(url, headers) {
        this.connection = new WebSocketConnection(url, headers);
        this.connection.setMsgHandler((event) => {
            this.buffer.push(JSON.parse(event.data));
        });
    }

    close() {
        this.connection.close();
    }

    on(type, handler) {
        if (type === "error")
            this.connection.setErrorHandler(handler);
    }

    async next() {
        for await (const next of this) return next;
    }

    async *[Symbol.asyncIterator]() {
        while (true) {
            if (this.buffer.length > 0) {
                yield this.buffer.shift();
            } else {
                if (this.connection.done) break;
                await this.connection.hasUpdate();
            }
        }
    }
}

export class OutStream {
    constructor(url, headers) {
        let responseResolver;
        this.responseValue = new Promise((resolve) => responseResolver = resolve);

        this.connection = new WebSocketConnection(url, headers);
        this.connection.setMsgHandler((event) => {
            responseResolver(JSON.parse(event.data))
        });
    }

    async response() {
        return this.responseValue;
    }

    close() {
        this.connection.close();
    }

    on(type, handler) {
        if (type === "error")
            this.connection.setErrorHandler(handler);
    }

    async send(msg) {
        if (this.connection.ws.readyState === WebSocket.CONNECTING) {
            // await that the socket is opened
            await new Promise((resolve) => {
                const handler = () => {
                    this.connection.ws.removeEventListener("open", handler);
                    resolve();
                };
                this.connection.ws.addEventListener("open", handler);
            });
        }

        return this.connection.ws.send(JSON.stringify(msg));
    }
}

const boundFetch = fetch.bind(this)

class BaseClient {
    constructor(baseURL, options) {
        this.baseURL = baseURL
        this.headers = {
            "Content-Type": "application/json",
        }

        // Add User-Agent header if the script is running in the server
        // because browsers do not allow setting User-Agent headers to requests
        if (typeof window === "undefined") {
            this.headers["User-Agent"] = "app-Generated-JS-Client (Encore/v0.0.0-develop)";
        }

        this.requestInit = options.requestInit ?? {}

        // Setup what fetch function we'll be using in the base client
        if (options.fetcher !== undefined) {
            this.fetcher = options.fetcher
        } else {
            this.fetcher = boundFetch
        }

        // Setup an authentication data generator using the auth data token option
        if (options.auth !== undefined) {
            const auth = options.auth
            if (typeof auth === "function") {
                this.authGenerator = auth
            } else {
                this.authGenerator = () => auth
            }
        }

    }

    async getAuthData() {
        let authData;

        // If authorization data generator is present, call it and add the returned data to the request
        if (this.authGenerator) {
            const mayBePromise = this.authGenerator();
            if (mayBePromise instanceof Promise) {
                authData = await mayBePromise;
            } else {
                authData = mayBePromise;
            }
        }

        if (authData) {
            const data = {};

            data.headers = {};
            data.headers["Authorization"] = "Bearer " + authData;

            return data;
        }
    }
    // createBidiStream sets up a stream to a streaming API endpoint.
    async createBidiStream(path, params) {
        let { query, headers } = params ?? {};

        // Fetch auth data if there is any
        const authData = await this.getAuthData();

        // If we now have authentication data, add it to the request
        if (authData) {
            if (authData.query) {
                query = {...query, ...authData.query};
            }
            if (authData.headers) {
                headers = {...headers, ...authData.headers};
            }
        }

        const queryString = query ? '?' + encodeQuery(query) : '';
        return new BidiStream(this.baseURL + path + queryString, headers);
    }

    // createInStream sets up a stream to a streaming API endpoint.
    async createInStream(path, params) {
        let { query, headers } = params ?? {};

        // Fetch auth data if there is any
        const authData = await this.getAuthData();

        // If we now have authentication data, add it to the request
        if (authData) {
            if (authData.query) {
                query = {...query, ...authData.query};
            }
            if (authData.headers) {
                headers = {...headers, ...authData.headers};
            }
        }

        const queryString = query ? '?' + encodeQuery(query) : ''
        return new InStream(this.baseURL + path + queryString, headers);
    }

    // createOutStream sets up a stream to a streaming API endpoint.
    async createOutStream(path, params) {
        let { query, headers } = params ?? {};

        // Fetch auth data if there is any
        const authData = await this.getAuthData();

        // If we now have authentication data, add it to the request
        if (authData) {
            if (authData.query) {
                query = {...query, ...authData.query};
            }
            if (authData.headers) {
                headers = {...headers, ...authData.headers};
            }
        }

        const queryString = query ? '?' + encodeQuery(query) : ''
        return new OutStream(this.baseURL + path + queryString, headers);
    }

    // callAPI is used by each generated API method to actually make the request
    async callAPI(method, path, body, params) {
        let { query, headers, ...rest } = params ?? {}
        const init = {
            ...this.requestInit,
            ...rest,
            method,
            body: body ?? null,
        }

        // Merge our headers with any predefined headers
        init.headers = {...this.headers, ...init.headers, ...headers}

        // Fetch auth data if there is any
        const authData = await this.getAuthData();

        // If we now have authentication data, add it to the request
        if (authData) {
            if (authData.query) {
                query = {...query, ...authData.query};
            }
            if (authData.headers) {
                init.headers = {...init.headers, ...authData.headers};
            }
        }

        // Make the actual request
        const queryString = query ? '?' + encodeQuery(query) : ''
        const response = await this.fetcher(this.baseURL+path+queryString, init)

        // handle any error responses
        if (!response.ok) {
            // try and get the error message from the response body
            let body = { code: ErrCode.Unknown, message: `request failed: status ${response.status}` }

            // if we can get the structured error we should, otherwise give a best effort
            try {
                const text = await response.text()

                try {
                    const jsonBody = JSON.parse(text)
                    if (isAPIErrorResponse(jsonBody)) {
                        body = jsonBody
                    } else {
                        body.message += ": " + JSON.stringify(jsonBody)
                    }
                } catch {
                    body.message += ": " + text
                }
            } catch (e) {
                // otherwise we just append the text to the error message
                body.message += ": " + String(e)
            }

            throw new APIError(response.status, body)
        }

        return response
    }
}

function isAPIErrorResponse(err) {
    return (
        err !== undefined && err !== null &&
        isErrCode(err.code) &&
        typeof(err.message) === "string" &&
        (err.details === undefined || err.details === null || typeof(err.details) === "object")
    )
}

function isErrCode(code) {
    return code !== undefined && Object.values(ErrCode).includes(code)
}

/**
 * APIError represents a structured error as returned from an Encore application.
 */
export class APIError extends Error {
    constructor(status, response) {
        // extending errors causes issues after you construct them, unless you apply the following fixes
        super(response.message);

        // set error name as constructor name, make it not enumerable to keep native Error behavior
        // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
        Object.defineProperty(this, 'name', {
            value:        'APIError',
            enumerable:   false,
            configurable: true,
        })

        // fix the prototype chain
        if (Object.setPrototypeOf == undefined) {
            this.__proto__ = APIError.prototype
        } else {
            Object.setPrototypeOf(this, APIError.prototype);
        }

        // capture a stack trace
        if (Error.captureStackTrace !== undefined) {
            Error.captureStackTrace(this, this.constructor);
        }

        /**
         * The HTTP status code associated with the error.
         */
        this.status = status

        /**
         * The Encore error code
         */
        this.code = response.code

        /**
         * The error details
         */
        this.details = response.details
    }
}

/**
 * Typeguard allowing use of an APIError's fields'
 */
export function isAPIError(err) {
    return err instanceof APIError;
}

export const ErrCode = {
    /**
     * OK indicates the operation was successful.
     */
    OK: "ok",

    /**
     * Canceled indicates the operation was canceled (typically by the caller).
     *
     * Encore will generate this error code when cancellation is requested.
     */
    Canceled: "canceled",

    /**
     * Unknown error. An example of where this error may be returned is
     * if a Status value received from another address space belongs to
     * an error-space that is not known in this address space. Also
     * errors raised by APIs that do not return enough error information
     * may be converted to this error.
     *
     * Encore will generate this error code in the above two mentioned cases.
     */
    Unknown: "unknown",

    /**
     * InvalidArgument indicates client specified an invalid argument.
     * Note that this differs from FailedPrecondition. It indicates arguments
     * that are problematic regardless of the state of the system
     * (e.g., a malformed file name).
     *
     * This error code will not be generated by the gRPC framework.
     */
    InvalidArgument: "invalid_argument",

    /**
     * DeadlineExceeded means operation expired before completion.
     * For operations that change the state of the system, this error may be
     * returned even if the operation has completed successfully. For
     * example, a successful response from a server could have been delayed
     * long enough for the deadline to expire.
     *
     * The gRPC framework will generate this error code when the deadline is
     * exceeded.
     */
    DeadlineExceeded: "deadline_exceeded",

    /**
     * NotFound means some requested entity (e.g., file or directory) was
     * not found.
     *
     * This error code will not be generated by the gRPC framework.
     */
    NotFound: "not_found",

    /**
     * AlreadyExists means an attempt to create an entity failed because one
     * already exists.
     *
     * This error code will not be generated by the gRPC framework.
     */
    AlreadyExists: "already_exists",

    /**
     * PermissionDenied indicates the caller does not have permission to
     * execute the specified operation. It must not be used for rejections
     * caused by exhausting some resource (use ResourceExhausted
     * instead for those errors). It must not be
     * used if the caller cannot be identified (use Unauthenticated
     * instead for those errors).
     *
     * This error code will not be generated by the gRPC core framework,
     * but expect authentication middleware to use it.
     */
    PermissionDenied: "permission_denied",

    /**
     * ResourceExhausted indicates some resource has been exhausted, perhaps
     * a per-user quota, or perhaps the entire file system is out of space.
     *
     * This error code will be generated by the gRPC framework in
     * out-of-memory and server overload situations, or when a message is
     * larger than the configured maximum size.
     */
    ResourceExhausted: "resource_exhausted",

    /**
     * FailedPrecondition indicates operation was rejected because the
     * system is not in a state required for the operation's execution.
     * For example, directory to be deleted may be non-empty, an rmdir
     * operation is applied to a non-directory, etc.
     *
     * A litmus test that may help a service implementor in deciding
     * between FailedPrecondition, Aborted, and Unavailable:
     *  (a) Use Unavailable if the client can retry just the failing call.
     *  (b) Use Aborted if the client should retry at a higher-level
     *      (e.g., restarting a read-modify-write sequence).
     *  (c) Use FailedPrecondition if the client should not retry until
     *      the system state has been explicitly fixed. E.g., if an "rmdir"
     *      fails because the directory is non-empty, FailedPrecondition
     *      should be returned since the client should not retry unless
     *      they have first fixed up the directory by deleting files from it.
     *  (d) Use FailedPrecondition if the client performs conditional
     *      REST Get/Update/Delete on a resource and the resource on the
     *      server does not match the condition. E.g., conflicting
     *      read-modify-write on the same resource.
     *
     * This error code will not be generated by the gRPC framework.
     */
    FailedPrecondition: "failed_precondition",

    /**
     * Aborted indicates the operation was aborted, typically due to a
     * concurrency issue like sequencer check failures, transaction aborts,
     * etc.
     *
     * See litmus test above for deciding between FailedPrecondition,
     * Aborted, and Unavailable.
     */
    Aborted: "aborted",

    /**
     * OutOfRange means operation was attempted past the valid range.
     * E.g., seeking or reading past end of file.
     *
     * Unlike InvalidArgument, this error indicates a problem that may
     * be fixed if the system state changes. For example, a 32-bit file
     * system will generate InvalidArgument if asked to read at an
     * offset that is not in the range [0,2^32-1], but it will generate
     * OutOfRange if asked to read from an offset past the current
     * file size.
     *
     * There is a fair bit of overlap between FailedPrecondition and
     * OutOfRange. We recommend using OutOfRange (the more specific
     * error) when it applies so that callers who are iterating through
     * a space can easily look for an OutOfRange error to detect when
     * they are done.
     *
     * This error code will not be generated by the gRPC framework.
     */
    OutOfRange: "out_of_range",

    /**
     * Unimplemented indicates operation is not implemented or not
     * supported/enabled in this service.
     *
     * This error code will be generated by the gRPC framework. Most
     * commonly, you will see this error code when a method implementation
     * is missing on the server. It can also be generated for unknown
     * compression algorithms or a disagreement as to whether an RPC should
     * be streaming.
     */
    Unimplemented: "unimplemented",

    /**
     * Internal errors. Means some invariants expected by underlying
     * system has been broken. If you see one of these errors,
     * something is very broken.
     *
     * This error code will be generated by the gRPC framework in several
     * internal error conditions.
     */
    Internal: "internal",

    /**
     * Unavailable indicates the service is currently unavailable.
     * This is a most likely a transient condition and may be corrected
     * by retrying with a backoff. Note that it is not always safe to retry
     * non-idempotent operations.
     *
     * See litmus test above for deciding between FailedPrecondition,
     * Aborted, and Unavailable.
     *
     * This error code will be generated by the gRPC framework during
     * abrupt shutdown of a server process or network connection.
     */
    Unavailable: "unavailable",

    /**
     * DataLoss indicates unrecoverable data loss or corruption.
     *
     * This error code will not be generated by the gRPC framework.
     */
    DataLoss: "data_loss",

    /**
     * Unauthenticated indicates the request does not have valid
     * authentication credentials for the operation.
     *
     * The gRPC framework will generate this error code when the
     * authentication metadata is invalid or a Credentials callback fails,
     * but also expect authentication middleware to generate it.
     */
    Unauthenticated: "unauthenticated"
}
