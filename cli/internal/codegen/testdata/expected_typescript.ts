
/**
 * BaseURL is the base URL for calling the Encore application's API.
 */
export type BaseURL = string

export const Local: BaseURL = "http://localhost:4000"

/**
 * Environment returns a BaseURL for calling the cloud environment with the given name.
 */
export function Environment(name: string): BaseURL {
    return `https://${name}-app.encr.app`
}

/**
 * Client is an API client for the app Encore application. 
 */
export default class Client {
    public readonly products: products.ServiceClient
    public readonly svc: svc.ServiceClient


    /**
     * @deprecated This constructor is deprecated, and you should move to using BaseURL with an Options object
     */
    constructor(target?: string, token?: string)

    /**
     * Creates a Client for calling the public and authenticated APIs of your Encore application.
     *
     * @param target  The target which the client should be configured to use. See Local and Environment for options.
     * @param options Options for the client
     */
    constructor(target: BaseURL, options?: ClientOptions)
    constructor(target: string | BaseURL = "prod", opts?: string | ClientOptions) {

        // Convert the old constructor parameters to a BaseURL object and a ClientOptions object
        if (!target.startsWith("http://") && !target.startsWith("https://")) {
            target = Environment(target)
        }

        if (typeof opts === "string") {
        	opts = { bearerToken: opts }
        } else {
        	opts ??= {}
        }

        const base = new BaseClient(target, opts)
        this.products = new products.ServiceClient(base)
        this.svc = new svc.ServiceClient(base)
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
     * Allows you to set the auth token to be used for each request
     * either by passing in a static token string or by passing in a function
     * which returns the auth token.
     *
     * These tokens will be sent as bearer tokens in the Authorization header.
     */
    bearerToken?: string | TokenGenerator
}

export namespace authentication {
    export interface User {
        id: number
        name: string
    }
}

export namespace products {
    export interface CreateProductRequest {
        IdempotencyKey: string
        name: string
        description: string
    }

    export interface Product {
        id: string
        name: string
        description: string
        "created_at": string
        "created_by": authentication.User
    }

    export interface ProductListing {
        products: Product[]
        previous: {
            cursor: string
            exists: boolean
        }
        next: {
            cursor: string
            exists: boolean
        }
    }

    export class ServiceClient {
        private baseClient: BaseClient

        constructor(baseClient: BaseClient) {
            this.baseClient = baseClient
        }

        public async Create(params: CreateProductRequest): Promise<Product> {
            // Convert our params into the objects we need for the request
            const headers: Record<string, string> = {
                "Idempotency-Key": params.IdempotencyKey,
            }

            // Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
            const body: Record<string, any> = {
                description: params.description,
                name:        params.name,
            }

            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/products.Create`, JSON.stringify(body), {headers})
            return await resp.json() as Product
        }

        public async List(): Promise<ProductListing> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("GET", `/products.List`)
            return await resp.json() as ProductListing
        }
    }
}

export namespace svc {
    export interface AllInputTypes<A> {
        /**
         * Specify this comes from a header field
         */
        A: string

        /**
         * Specify this comes from a query string
         */
        B: number[]

        /**
         * This can come from anywhere, but if it comes from the payload in JSON it must be called Charile
         */
        "Charlies-Bool": boolean

        /**
         * This generic type complicates the whole thing 🙈
         */
        Dave: A
    }

    export type Foo = number

    export interface GetRequest {
        Bar: string
        Baz: number
    }

    /**
     * HeaderOnlyStruct contains all types we support in headers
     */
    export interface HeaderOnlyStruct {
        Boolean: boolean
        Int: number
        Float: number
        String: string
        Bytes: string
        Time: string
        Json: JSONValue
        UUID: string
        UserID: string
    }

    export interface Request {
        /**
         * Foo is good
         */
        Foo?: Foo

        /**
         * Baz is better
         */
        boo: string

        /**
         * This is a multiline
         * comment on the raw message!
         */
        Raw: JSONValue
    }

    /**
     * Tuple is a generic type which allows us to
     * return two values of two different types
     */
    export interface Tuple<A, B> {
        A: A
        B: B
    }

    export type WrappedRequest = Wrapper<Request>

    export interface Wrapper<T> {
        Value: T
    }

    export class ServiceClient {
        private baseClient: BaseClient

        constructor(baseClient: BaseClient) {
            this.baseClient = baseClient
        }

        /**
         * DummyAPI is a dummy endpoint.
         */
        public async DummyAPI(params: Request): Promise<void> {
            await this.baseClient.callAPI("POST", `/svc.DummyAPI`, JSON.stringify(params))
        }

        public async Get(params: GetRequest): Promise<void> {
            // Convert our params into the objects we need for the request
            const query: Record<string, any> = {
                boo: params.Baz,
            }

            await this.baseClient.callAPI("GET", `/svc.Get?${encodeQuery(query)}`)
        }

        public async GetRequestWithAllInputTypes(params: AllInputTypes<number>): Promise<HeaderOnlyStruct> {
            // Convert our params into the objects we need for the request
            const headers: Record<string, string> = {
                "X-Alice": String(params.A),
            }

            const query: Record<string, any> = {
                Bob:  params.B,
                c:    params["Charlies-Bool"],
                dave: params.Dave,
            }

            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("GET", `/svc.GetRequestWithAllInputTypes?${encodeQuery(query)}`, undefined, {headers})

            //Populate the return object from the JSON body and received headers
            const rtn = await resp.json() as HeaderOnlyStruct
            rtn.Boolean = resp.headers.get("x-boolean").toLowerCase() === "true"
            rtn.Int = parseInt(resp.headers.get("x-int"), 10)
            rtn.Float = Number(resp.headers.get("x-float"))
            rtn.String = resp.headers.get("x-string")
            rtn.Bytes = resp.headers.get("x-bytes")
            rtn.Time = resp.headers.get("x-time")
            rtn.Json = JSON.parse(resp.headers.get("x-json"))
            rtn.UUID = resp.headers.get("x-uuid")
            rtn.UserID = resp.headers.get("x-user-id")
            return rtn
        }

        public async HeaderOnlyRequest(params: HeaderOnlyStruct): Promise<void> {
            // Convert our params into the objects we need for the request
            const headers: Record<string, string> = {
                "x-boolean": String(params.Boolean),
                "x-bytes":   String(params.Bytes),
                "x-float":   String(params.Float),
                "x-int":     String(params.Int),
                "x-json":    String(params.Json),
                "x-string":  params.String,
                "x-time":    String(params.Time),
                "x-user-id": String(params.UserID),
                "x-uuid":    String(params.UUID),
            }

            await this.baseClient.callAPI("GET", `/svc.HeaderOnlyRequest`, undefined, {headers})
        }

        public async RESTPath(a: string, b: number): Promise<void> {
            await this.baseClient.callAPI("POST", `/path/${a}/${b}`)
        }

        public async RequestWithAllInputTypes(params: AllInputTypes<string>): Promise<AllInputTypes<number>> {
            // Convert our params into the objects we need for the request
            const headers: Record<string, string> = {
                "X-Alice": String(params.A),
            }

            const query: Record<string, any> = {
                Bob: params.B,
            }

            // Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
            const body: Record<string, any> = {
                "Charlies-Bool": params["Charlies-Bool"],
                Dave:            params.Dave,
            }

            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/svc.RequestWithAllInputTypes?${encodeQuery(query)}`, JSON.stringify(body), {headers})

            //Populate the return object from the JSON body and received headers
            const rtn = await resp.json() as AllInputTypes<number>
            rtn.A = resp.headers.get("X-Alice")
            return rtn
        }

        /**
         * TupleInputOutput tests the usage of generics in the client generator
         * and this comment is also multiline, so multiline comments get tested as well.
         */
        public async TupleInputOutput(params: Tuple<string, WrappedRequest>): Promise<Tuple<boolean, Foo>> {
            // Now make the actual call to the API
            const resp = await this.baseClient.callAPI("POST", `/svc.TupleInputOutput`, JSON.stringify(params))
            return await resp.json() as Tuple<boolean, Foo>
        }

        public async Webhook(a: string, b: string, body?: BodyInit, options?: CallParameters): Promise<Response> {
            return this.baseClient.callAPI("POST", `/webhook/${a}/${b}`, body, options)
        }
    }
}

// JSONValue represents an arbitrary JSON value.
export type JSONValue = string | number | boolean | null | JSONValue[] | {[key: string]: JSONValue}

// CallParameters is the type of the parameters to a method call, but require headers to be a Record type
type CallParameters = Omit<RequestInit, "method" | "body"> & { headers?: Record<string, string> }

// TokenGenerator is a function that returns a token
export type TokenGenerator = () => string

// A fetcher is the prototype for the inbuilt Fetch function
export type Fetcher = (input: RequestInfo, init?: RequestInit) => Promise<Response>;

class BaseClient {
    readonly baseURL: string
    readonly fetcher: Fetcher
    readonly headers: Record<string, string>
    readonly tokenGenerator?: TokenGenerator

    constructor(baseURL: string, options: ClientOptions) {
        this.baseURL = baseURL
        this.headers = {
            "Content-Type": "application/json",
            "User-Agent":   "app-Generated-TS-Client (Encore/devel)",
        }

        // Setup what fetch function we'll be using in the base client
        if (options.fetcher !== undefined) {
            this.fetcher = options.fetcher
        } else {
            this.fetcher = fetch
        }

        // Setup a token generator using the bearer token option
        if (options.bearerToken !== undefined) {
            const token = options.bearerToken
            if (typeof token === "string") {
                this.tokenGenerator = () => token
            } else {
                this.tokenGenerator = token
            }
        }
    }

    // callAPI is used by each generated API method to actually make the request
    public async callAPI(method: string, path: string, body?: BodyInit, params?: CallParameters): Promise<Response> {
        const init: RequestInit = {
            method,
            body,
            ...(params ?? {}),
        }

        // Merge our headers with any predefined headers
        init.headers = {...this.headers, ...init.headers}

        // If an authorization token generator is present, call it and add the returned token to the request
        if (this.tokenGenerator) {
            init.headers["Authorization"] = "Bearer " + this.tokenGenerator()
        }

        // Make the actual request
        const response = await this.fetcher(this.baseURL + path, init)

        // handle any error responses
        if (!response.ok) {
            const body = await response.text()
            throw new Error(`request failed: status ${response.status}: ${body}`)
        }

        return response
    }
}

function encodeQuery(parts: Record<string, any>): string {
    const pairs = []
    for (let key in parts) {
        let val = parts[key]
        if (!Array.isArray(val)) {
            val = [val]
        }
        for (const v of val) {
            pairs.push(`${key}=${encodeURIComponent(v)}`)
        }
    }
    return pairs.join("&")
}