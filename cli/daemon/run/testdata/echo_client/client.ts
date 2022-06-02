// Code generated by the Encore devel client generator. DO NOT EDIT.

/* eslint-disable @typescript-eslint/no-namespace */
/* eslint-disable @typescript-eslint/no-explicit-any */

/**
 * BaseURL is the base URL for calling the Encore application's API.
 */
export type BaseURL = string

export const Local: BaseURL = "http://localhost:4000"

/**
 * Environment returns a BaseURL for calling the cloud environment with the given name.
 */
export function Environment(name: string): BaseURL {
    return `https://${name}-slug.encr.app`
}

/**
 * Client is an API client for the slug Encore application. 
 */
export default class Client {
    public readonly echo: echo.ServiceClient
    public readonly endtoend: endtoend.ServiceClient
    public readonly test: test.ServiceClient


    /**
     * Creates a Client for calling the public and authenticated APIs of your Encore application.
     *
     * @param target  The target which the client should be configured to use. See Local and Environment for options.
     * @param options Options for the client
     */
    constructor(target: BaseURL, options?: ClientOptions) {
        const base = new BaseClient(target, options ?? {})
        this.echo = new echo.ServiceClient(base)
        this.endtoend = new endtoend.ServiceClient(base)
        this.test = new test.ServiceClient(base)
    }
}

/**
 * ClientOptions allows you to override any default behaviour within the generated Encore client.
 */
export interface ClientOptions {
    /**
     * By default the client will use the inbuilt fetch function for making the API requests.
     * however you can override it with your own implementation here if you want to run custom
     * code on each API request made or response received.
     */
    fetcher?: Fetcher

    /**
     * Allows you to set the authentication data to be used for each
     * request either by passing in a static object or by passing in
     * a function which returns a new object for each request.
     */
    auth?: echo.AuthParams | AuthDataGenerator
}

export namespace echo {
    export interface AppMetadata {
        AppID: string
        APIBaseURL: string
        EnvName: string
        EnvType: string
    }

    export interface AuthParams {
        Header: string
        AuthInt: number
        Authorization: string
        Query: number[]
        NewAuth: boolean
    }

    export interface BasicData {
        String: string
        Uint: number
        Int: number
        Int8: number
        Int64: number
        Float32: number
        Float64: number
        StringSlice: string[]
        IntSlice: number[]
        Time: string
    }

    export interface Data<K, V> {
        Key: K
        Value: V
    }

    export interface EmptyData {
        OmitEmpty: Data<string, string>
        NullPtr: string
        Zero: Data<string, string>
    }

    export interface EnvResponse {
        Env: string[]
    }

    export interface HeadersData {
        Int: number
        String: string
    }

    export interface NonBasicData {
        /**
         * Header
         */
        HeaderString: string

        HeaderNumber: number
        /**
         * Body
         */
        Struct: Data<Data<string, string>, number>

        StructPtr: Data<number, number>
        StructSlice: Data<string, string>[]
        StructMap: { [key: string]: Data<string, number> }
        StructMapPtr: { [key: string]: Data<string, string> }
        AnonStruct: {
            AnonBird: string
        }
        "formatted_nest": Data<string, number>
        RawStruct: JSONValue
        /**
         * Query
         */
        QueryString: string

        QueryNumber: number
        /**
         * Path Parameters
         */
        PathString: string

        PathInt: number
        PathWild: string
        /**
         * Auth Parameters
         */
        AuthHeader: string

        AuthQuery: number[]
    }

    export class ServiceClient {
        private baseClient: BaseClient

        constructor(baseClient: BaseClient) {
            this.baseClient = baseClient
        }

        /**
         * AppMeta returns app metadata.
         */
        public async AppMeta(): Promise<AppMetadata> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/echo.AppMeta`)
            return await resp.json() as AppMetadata
        }

        /**
         * BasicEcho echoes back the request data.
         */
        public async BasicEcho(params: BasicData): Promise<BasicData> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/echo.BasicEcho`, JSON.stringify(params))
            return await resp.json() as BasicData
        }

        /**
         * Echo echoes back the request data.
         */
        public async Echo(params: Data<string, number>): Promise<Data<string, number>> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/echo.Echo`, JSON.stringify(params))
            return await resp.json() as Data<string, number>
        }

        /**
         * EmptyEcho echoes back the request data.
         */
        public async EmptyEcho(params: EmptyData): Promise<EmptyData> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/echo.EmptyEcho`, JSON.stringify(params))
            return await resp.json() as EmptyData
        }

        /**
         * Env returns the environment.
         */
        public async Env(): Promise<EnvResponse> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/echo.Env`)
            return await resp.json() as EnvResponse
        }

        /**
         * HeadersEcho echoes back the request headers
         */
        public async HeadersEcho(params: HeadersData): Promise<HeadersData> {
            // Convert our params into the objects we need for the request
            const headers: Record<string, string> = {
                "x-int":    String(params.Int),
                "x-string": params.String,
            }

            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/echo.HeadersEcho`, undefined, {headers})

            //Populate the return object from the JSON body and received headers
            const rtn = await resp.json() as HeadersData
            rtn.Int = parseInt(mustBeSet("Header `x-int`", resp.headers.get("x-int")), 10)
            rtn.String = mustBeSet("Header `x-string`", resp.headers.get("x-string"))
            return rtn
        }

        /**
         * MuteEcho absorbs a request
         */
        public async MuteEcho(params: Data<string, string>): Promise<void> {
            // Convert our params into the objects we need for the request
            const query: Record<string, string | string[]> = {
                key:   params.Key,
                value: params.Value,
            }

            await this.baseClient.callAPI("GET", `/echo.MuteEcho`, undefined, {query})
        }

        /**
         * NilResponse returns a nil response and nil error
         */
        public async NilResponse(): Promise<BasicData> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/echo.NilResponse`)
            return await resp.json() as BasicData
        }

        /**
         * NonBasicEcho echoes back the request data.
         */
        public async NonBasicEcho(pathString: string, pathInt: number, pathWild: string[], params: NonBasicData): Promise<NonBasicData> {
            // Convert our params into the objects we need for the request
            const headers: Record<string, string> = {
                "x-header-number": String(params.HeaderNumber),
                "x-header-string": params.HeaderString,
            }

            const query: Record<string, string | string[]> = {
                no:     String(params.QueryNumber),
                string: params.QueryString,
            }

            // Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
            const body: Record<string, any> = {
                AnonStruct:       params.AnonStruct,
                AuthHeader:       params.AuthHeader,
                AuthQuery:        params.AuthQuery,
                PathInt:          params.PathInt,
                PathString:       params.PathString,
                PathWild:         params.PathWild,
                RawStruct:        params.RawStruct,
                Struct:           params.Struct,
                StructMap:        params.StructMap,
                StructMapPtr:     params.StructMapPtr,
                StructPtr:        params.StructPtr,
                StructSlice:      params.StructSlice,
                "formatted_nest": params["formatted_nest"],
            }

            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/NonBasicEcho/${encodeURIComponent(pathString)}/${encodeURIComponent(pathInt)}/${pathWild.map(encodeURIComponent).join("/")}`, JSON.stringify(body), {headers, query})

            //Populate the return object from the JSON body and received headers
            const rtn = await resp.json() as NonBasicData
            rtn.HeaderString = mustBeSet("Header `x-header-string`", resp.headers.get("x-header-string"))
            rtn.HeaderNumber = parseInt(mustBeSet("Header `x-header-number`", resp.headers.get("x-header-number")), 10)
            return rtn
        }

        /**
         * Noop does nothing
         */
        public async Noop(): Promise<void> {
            await this.baseClient.callAPI("GET", `/echo.Noop`)
        }

        /**
         * Pong returns a bird tuple
         */
        public async Pong(): Promise<Data<string, string>> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("GET", `/echo.Pong`)
            return await resp.json() as Data<string, string>
        }
    }
}

export namespace endtoend {

    export class ServiceClient {
        private baseClient: BaseClient

        constructor(baseClient: BaseClient) {
            this.baseClient = baseClient
        }

        public async GeneratedWrappersEndToEndTest(): Promise<void> {
            await this.baseClient.callAPI("GET", `/generated-wrappers-end-to-end-test`)
        }
    }
}

export namespace test {
    export interface BodyEcho {
        Message: string
    }

    export interface MarshallerTest<A> {
        HeaderBoolean: boolean
        HeaderInt: number
        HeaderFloat: number
        HeaderString: string
        HeaderBytes: string
        HeaderTime: string
        HeaderJson: JSONValue
        HeaderUUID: string
        HeaderUserID: string
        QueryBoolean: boolean
        QueryInt: number
        QueryFloat: number
        QueryString: string
        QueryBytes: string
        QueryTime: string
        QueryJson: JSONValue
        QueryUUID: string
        QueryUserID: string
        QuerySlice: A[]
        boolean: boolean
        int: number
        float: number
        string: string
        bytes: string
        time: string
        json: JSONValue
        uuid: string
        "user-id": string
        slice: A[]
    }

    export interface MultiPathSegment {
        Boolean: boolean
        Int: number
        String: string
        UUID: string
        Wildcard: string
    }

    export interface RestParams {
        HeaderValue: string
        QueryValue: string
        "Some-Key": string
        Nested: {
            Alice: string
            bOb: number
            charile: boolean
        }
    }

    export class ServiceClient {
        private baseClient: BaseClient

        constructor(baseClient: BaseClient) {
            this.baseClient = baseClient
        }

        /**
         * GetMessage allows us to test an API which takes no parameters,
         * but returns data. It also tests two API's on the same path with different HTTP methods
         */
        public async GetMessage(clientID: string): Promise<BodyEcho> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("GET", `/last_message/${encodeURIComponent(clientID)}`)
            return await resp.json() as BodyEcho
        }

        /**
         * MarshallerTestHandler allows us to test marshalling of all the inbuilt types in all
         * the field types. It simply echos all the responses back to the client
         */
        public async MarshallerTestHandler(params: MarshallerTest<number>): Promise<MarshallerTest<number>> {
            // Convert our params into the objects we need for the request
            const headers: Record<string, string> = {
                "x-boolean": String(params.HeaderBoolean),
                "x-bytes":   String(params.HeaderBytes),
                "x-float":   String(params.HeaderFloat),
                "x-int":     String(params.HeaderInt),
                "x-json":    JSON.stringify(params.HeaderJson),
                "x-string":  params.HeaderString,
                "x-time":    String(params.HeaderTime),
                "x-user-id": String(params.HeaderUserID),
                "x-uuid":    String(params.HeaderUUID),
            }

            const query: Record<string, string | string[]> = {
                boolean:   String(params.QueryBoolean),
                bytes:     String(params.QueryBytes),
                float:     String(params.QueryFloat),
                int:       String(params.QueryInt),
                json:      JSON.stringify(params.QueryJson),
                slice:     params.QuerySlice.map((v) => String(v)),
                string:    params.QueryString,
                time:      String(params.QueryTime),
                "user-id": String(params.QueryUserID),
                uuid:      String(params.QueryUUID),
            }

            // Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
            const body: Record<string, any> = {
                boolean:   params.boolean,
                bytes:     params.bytes,
                float:     params.float,
                int:       params.int,
                json:      params.json,
                slice:     params.slice,
                string:    params.string,
                time:      params.time,
                "user-id": params["user-id"],
                uuid:      params.uuid,
            }

            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/test.MarshallerTestHandler`, JSON.stringify(body), {headers, query})

            //Populate the return object from the JSON body and received headers
            const rtn = await resp.json() as MarshallerTest<number>
            rtn.HeaderBoolean = mustBeSet("Header `x-boolean`", resp.headers.get("x-boolean")).toLowerCase() === "true"
            rtn.HeaderInt = parseInt(mustBeSet("Header `x-int`", resp.headers.get("x-int")), 10)
            rtn.HeaderFloat = Number(mustBeSet("Header `x-float`", resp.headers.get("x-float")))
            rtn.HeaderString = mustBeSet("Header `x-string`", resp.headers.get("x-string"))
            rtn.HeaderBytes = mustBeSet("Header `x-bytes`", resp.headers.get("x-bytes"))
            rtn.HeaderTime = mustBeSet("Header `x-time`", resp.headers.get("x-time"))
            rtn.HeaderJson = JSON.parse(mustBeSet("Header `x-json`", resp.headers.get("x-json")))
            rtn.HeaderUUID = mustBeSet("Header `x-uuid`", resp.headers.get("x-uuid"))
            rtn.HeaderUserID = mustBeSet("Header `x-user-id`", resp.headers.get("x-user-id"))
            return rtn
        }

        /**
         * Noop allows us to test if a simple HTTP request can be made
         */
        public async Noop(): Promise<void> {
            await this.baseClient.callAPI("POST", `/test.Noop`)
        }

        /**
         * NoopWithError allows us to test if the structured errors are returned
         */
        public async NoopWithError(): Promise<void> {
            await this.baseClient.callAPI("POST", `/test.NoopWithError`)
        }

        /**
         * PathMultiSegments allows us to wildcard segments and segment URI encoding
         */
        public async PathMultiSegments(bool: boolean, int: number, _string: string, uuid: string, wildcard: string[]): Promise<MultiPathSegment> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/multi/${encodeURIComponent(bool)}/${encodeURIComponent(int)}/${encodeURIComponent(_string)}/${encodeURIComponent(uuid)}/${wildcard.map(encodeURIComponent).join("/")}`)
            return await resp.json() as MultiPathSegment
        }

        /**
         * RawEndpoint allows us to test the clients' ability to send raw requests
         * under auth
         */
        public async RawEndpoint(method: "PUT" | "POST" | "DELETE" | "GET", id: string[], body?: BodyInit, options?: CallParameters): Promise<Response> {
            return this.baseClient.callAPI(method, `/raw/blah/${id.map(encodeURIComponent).join("/")}`, body, options)
        }

        /**
         * RestStyleAPI tests all the ways we can get data into and out of the application
         * using Encore request handlers
         */
        public async RestStyleAPI(objType: number, name: string, params: RestParams): Promise<RestParams> {
            // Convert our params into the objects we need for the request
            const headers: Record<string, string> = {
                "some-key": params.HeaderValue,
            }

            const query: Record<string, string | string[]> = {
                "Some-Key": params.QueryValue,
            }

            // Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
            const body: Record<string, any> = {
                Nested:     params.Nested,
                "Some-Key": params["Some-Key"],
            }

            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("PUT", `/rest/object/${encodeURIComponent(objType)}/${encodeURIComponent(name)}`, JSON.stringify(body), {headers, query})

            //Populate the return object from the JSON body and received headers
            const rtn = await resp.json() as RestParams
            rtn.HeaderValue = mustBeSet("Header `some-key`", resp.headers.get("some-key"))
            return rtn
        }

        /**
         * SimpleBodyEcho allows us to exercise the body marshalling from JSON
         * and being returned purely as a body
         */
        public async SimpleBodyEcho(params: BodyEcho): Promise<BodyEcho> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/test.SimpleBodyEcho`, JSON.stringify(params))
            return await resp.json() as BodyEcho
        }

        /**
         * TestAuthHandler allows us to test the clients ability to add tokens to requests
         */
        public async TestAuthHandler(): Promise<BodyEcho> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/test.TestAuthHandler`)
            return await resp.json() as BodyEcho
        }

        /**
         * UpdateMessage allows us to test an API which takes parameters,
         * but doesn't return anything
         */
        public async UpdateMessage(clientID: string, params: BodyEcho): Promise<void> {
            await this.baseClient.callAPI("PUT", `/last_message/${encodeURIComponent(clientID)}`, JSON.stringify(params))
        }
    }
}

// JSONValue represents an arbitrary JSON value.
export type JSONValue = string | number | boolean | null | JSONValue[] | {[key: string]: JSONValue}


function encodeQuery(parts: Record<string, string | string[]>): string {
    const pairs = []
    for (const key in parts) {
        const val = (Array.isArray(parts[key]) ?  parts[key] : [parts[key]]) as string[]
        for (const v of val) {
            pairs.push(`${key}=${encodeURIComponent(v)}`)
        }
    }
    return pairs.join("&")
}

// mustBeSet will throw an APIError with the Data Loss code if value is null or undefined
function mustBeSet<A>(field: string, value: A | null | undefined): A {
    if (value === null || value === undefined) {
        throw new APIError(
            500,
            {
                code: ErrCode.DataLoss,
                message: `${field} was unexpectedly ${value}`, // ${value} will create the string "null" or "undefined"
            },
        )
    }
    return value
}

// CallParameters is the type of the parameters to a method call, but require headers to be a Record type
type CallParameters = Omit<RequestInit, "method" | "body"> & {
    /** Any headers to be sent with the request */
    headers?: Record<string, string>;

    /** Any query parameters to be sent with the request */
    query?: Record<string, string | string[]>
}

// AuthDataGenerator is a function that returns a new instance of the authentication data required by this API
export type AuthDataGenerator = () => echo.AuthParams

// A fetcher is the prototype for the inbuilt Fetch function
export type Fetcher = (input: RequestInfo, init?: RequestInit) => Promise<Response>;

class BaseClient {
    readonly baseURL: string
    readonly fetcher: Fetcher
    readonly headers: Record<string, string>
    readonly authGenerator?: () => echo.AuthParams

    constructor(baseURL: string, options: ClientOptions) {
        this.baseURL = baseURL
        this.headers = {
            "Content-Type": "application/json",
            "User-Agent":   "slug-Generated-TS-Client (Encore/devel)",
        }

        // Setup what fetch function we'll be using in the base client
        if (options.fetcher !== undefined) {
            this.fetcher = options.fetcher
        } else {
            this.fetcher = fetch
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

    // callAPI is used by each generated API method to actually make the request
    public async callAPI(method: string, path: string, body?: BodyInit, params?: CallParameters): Promise<Response> {
        // eslint-disable-next-line prefer-const
        let { query, ...rest } = params ?? {}
        const init = {
            ...rest,
            method,
            body: body ?? null,
        }

        // Merge our headers with any predefined headers
        init.headers = {...this.headers, ...init.headers}

        // If authorization data generator is present, call it and add the returned data to the request
        let authData: echo.AuthParams | undefined
        if (this.authGenerator) {
            authData = this.authGenerator()
        }

        // If we now have authentication data, add it to the request
        if (authData) {
            query = query ?? {}
            query["query"] = authData.Query.map((v) => String(v))
            query["new-auth"] = String(authData.NewAuth)
            init.headers["x-header"] = authData.Header
            init.headers["x-auth-int"] = String(authData.AuthInt)
            init.headers["authorization"] = authData.Authorization
        }

        // Make the actual request
        const queryString = query ? '?' + encodeQuery(query) : ''
        const response = await this.fetcher(this.baseURL+path+queryString, init)

        // handle any error responses
        if (!response.ok) {
            // try and get the error message from the response body
            let body: APIErrorResponse = { code: ErrCode.Unknown, message: `request failed: status ${response.status}` }

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

/**
 * APIErrorDetails represents the response from an Encore API in the case of an error
 */
interface APIErrorResponse {
    code: ErrCode
    message: string
    details?: any
}

function isAPIErrorResponse(err: any): err is APIErrorResponse {
    return (
        err !== undefined && err !== null && 
        isErrCode(err.code) &&
        typeof(err.message) === "string" &&
        (err.details === undefined || err.details === null || typeof(err.details) === "object")
    )
}

function isErrCode(code: any): code is ErrCode {
    return code !== undefined && Object.values(ErrCode).includes(code)
}

/**
 * APIError represents a structured error as returned from an Encore application.
 */
export class APIError extends Error {
    /**
     * The HTTP status code associated with the error.
     */
    public readonly status: number

    /**
     * The Encore error code
     */
    public readonly code: ErrCode

    /**
     * The error details
     */
    public readonly details?: any

    constructor(status: number, response: APIErrorResponse) {
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
        if ((Object as any).setPrototypeOf == undefined) { 
            (this as any).__proto__ = APIError.prototype 
        } else {
            Object.setPrototypeOf(this, APIError.prototype);
        }
        
        // capture a stack trace
        if ((Error as any).captureStackTrace !== undefined) {
            (Error as any).captureStackTrace(this, this.constructor);
        }

        this.status = status
        this.code = response.code
        this.details = response.details
    }
}

/**
 * Typeguard allowing use of an APIError's fields'
 */
export function isAPIError(err: any): err is APIError {
    return err instanceof APIError;
}

export enum ErrCode {
    /**
     * OK indicates the operation was successful.
     */
    OK = "ok",

    /**
     * Canceled indicates the operation was canceled (typically by the caller).
     *
     * Encore will generate this error code when cancellation is requested.
     */
    Canceled = "canceled",

    /**
     * Unknown error. An example of where this error may be returned is
     * if a Status value received from another address space belongs to
     * an error-space that is not known in this address space. Also
     * errors raised by APIs that do not return enough error information
     * may be converted to this error.
     *
     * Encore will generate this error code in the above two mentioned cases.
     */
    Unknown = "unknown",

    /**
     * InvalidArgument indicates client specified an invalid argument.
     * Note that this differs from FailedPrecondition. It indicates arguments
     * that are problematic regardless of the state of the system
     * (e.g., a malformed file name).
     *
     * This error code will not be generated by the gRPC framework.
     */
    InvalidArgument = "invalid_argument",

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
    DeadlineExceeded = "deadline_exceeded",

    /**
     * NotFound means some requested entity (e.g., file or directory) was
     * not found.
     *
     * This error code will not be generated by the gRPC framework.
     */
    NotFound = "not_found",

    /**
     * AlreadyExists means an attempt to create an entity failed because one
     * already exists.
     *
     * This error code will not be generated by the gRPC framework.
     */
    AlreadyExists = "already_exists",

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
    PermissionDenied = "permission_denied",

    /**
     * ResourceExhausted indicates some resource has been exhausted, perhaps
     * a per-user quota, or perhaps the entire file system is out of space.
     *
     * This error code will be generated by the gRPC framework in
     * out-of-memory and server overload situations, or when a message is
     * larger than the configured maximum size.
     */
    ResourceExhausted = "resource_exhausted",

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
    FailedPrecondition = "failed_precondition",

    /**
     * Aborted indicates the operation was aborted, typically due to a
     * concurrency issue like sequencer check failures, transaction aborts,
     * etc.
     *
     * See litmus test above for deciding between FailedPrecondition,
     * Aborted, and Unavailable.
     */
    Aborted = "aborted",

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
    OutOfRange = "out_of_range",

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
    Unimplemented = "unimplemented",

    /**
     * Internal errors. Means some invariants expected by underlying
     * system has been broken. If you see one of these errors,
     * something is very broken.
     *
     * This error code will be generated by the gRPC framework in several
     * internal error conditions.
     */
    Internal = "internal",

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
    Unavailable = "unavailable",

    /**
     * DataLoss indicates unrecoverable data loss or corruption.
     *
     * This error code will not be generated by the gRPC framework.
     */
    DataLoss = "data_loss",

    /**
     * Unauthenticated indicates the request does not have valid
     * authentication credentials for the operation.
     *
     * The gRPC framework will generate this error code when the
     * authentication metadata is invalid or a Credentials callback fails,
     * but also expect authentication middleware to generate it.
     */
    Unauthenticated = "unauthenticated",
}
