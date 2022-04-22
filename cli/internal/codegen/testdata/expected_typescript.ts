export default class Client {
    svc: svc.ServiceClient

    constructor(environment: string = "prod", token?: string) {
        const base = new BaseClient(environment, token)
        this.svc = new svc.ServiceClient(base)
    }
}

export namespace svc {
    export type Foo = number

    export interface GetRequest {
        Bar: string
        Baz: string
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

    export type Wrapper<T> = T

    export class ServiceClient {
        private baseClient: BaseClient

        constructor(baseClient: BaseClient) {
            this.baseClient = baseClient
        }

        public DummyAPI(params: Request): Promise<void> {
            return this.baseClient.doVoid("POST", `/svc.DummyAPI`, params)
        }

        /**
         * Get returns some stuff
         */
        public Get(params: GetRequest): Promise<void> {
            const query: any[] = [
                "boo", params.Baz,
            ]
            return this.baseClient.doVoid("GET", `/svc.Get?${encodeQuery(query)}`)
        }

        public RESTPath(a: string, b: number): Promise<void> {
            return this.baseClient.doVoid("GET", `/path/${a}/${b}`)
        }

        /**
         * TupleInputOutput tests the usage of generics in the client generator
         * and this comment is also multiline, so multiline comments get tested as well.
         */
        public TupleInputOutput(params: Tuple<string, WrappedRequest>): Promise<Tuple<boolean, Foo>> {
            return this.baseClient.do<Tuple<boolean, Foo>>("POST", `/svc.TupleInputOutput`, params)
        }

        public Webhook(a: string, b: string): Promise<void> {
            return this.baseClient.doVoid("POST", `/webhook/${a}/${b}`)
        }
    }
}

// JSONValue represents an arbitrary JSON value.
export type JSONValue = string | number | boolean | null | JSONValue[] | {[key: string]: JSONValue}

class BaseClient {
    baseURL: string
    headers: {[key: string]: string}

    constructor(environment: string, token?: string) {
        this.headers = {"Content-Type": "application/json"}
        if (token !== undefined) {
            this.headers["Authorization"] = "Bearer " + token
        }
        if (environment === "local") {
            this.baseURL = "http://localhost:4000"
        } else {
            this.baseURL = `https://app.encoreapi.com/${environment}`
        }
    }

    public async do<T>(method: string, path: string, req?: any): Promise<T> {
        let response = await fetch(this.baseURL + path, {
            method: method,
            headers: this.headers,
            body: req !== undefined ? JSON.stringify(req) : undefined
        })
        if (!response.ok) {
            let body = await response.text()
            throw new Error("request failed: " + body)
        }
        return <T>(await response.json())
    }

    public async doVoid(method: string, path: string, req?: any): Promise<void> {
        let response = await fetch(this.baseURL + path, {
            method: method,
            headers: this.headers,
            body: req !== undefined ? JSON.stringify(req) : undefined
        })
        if (!response.ok) {
            let body = await response.text()
            throw new Error("request failed: " + body)
        }
        await response.text()
    }
}

function encodeQuery(parts: any[]): string {
    const pairs = []
    for (let i = 0; i < parts.length; i += 2) {
        const key = parts[i]
        let val = parts[i+1]
        if (!Array.isArray(val)) {
            val = [val]
        }
        for (const v of val) {
            pairs.push(`${key}=${encodeURIComponent(v)}`)
        }
    }
    return pairs.join("&")
}
