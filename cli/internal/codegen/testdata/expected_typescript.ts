export interface ErrorResponse {
    code: number,
    message: string,
    details: any
}

export type Result<T> = { data: T } | { error: ErrorResponse }

export default class Client {
    svc: svc.ServiceClient

    public async doRaw(method: string, path: string, body?: any): Promise<Response> {
        const headers: Record<string, string> = { "Content-Type": "application/json" }
        if (this.token) {
            headers["Authorization"] = "Bearer " + this.token
        }
        return fetch(this.baseURL + path, {
            method,
            headers,
            body
        })
    }

    public async do<T>(method: string, path: string, req?: any): Promise<Result<T>> {
        try {
            const response = await this.doRaw(method, path, req !== undefined ? JSON.stringify(req) : undefined)
            if (!response.ok) {
                const error = <ErrorResponse>(await response.json())
                return { error }
            }
            return { data: <T>(await response.json().catch(_ => null)) }
        } catch (error) {
            return {
                error: <ErrorResponse>{
                    code: -1,
                    message: error.message
                }
            }
        }
    }

    public async doVoid(method: string, path: string, req?: any): Promise<ErrorResponse | null> {
        try {
            const response = await this.doRaw(method, path, req !== undefined ? JSON.stringify(req) : undefined)
            if (!response.ok) {
                const error = <ErrorResponse>(await response.json())
                return error
            }
            return null

        } catch (error) {
            return <ErrorResponse>{
                code: -1,
                message: error.message

            }
        }
    }

    protected baseURL: string

    constructor(environment: string = "prod", public token?: string) {
        if (environment.startsWith('http://') || environment.startsWith('https://')) {
            this.baseURL = environment
        } else {
            this.baseURL = environment === "local" ? "http://localhost:4000" : `https://app.encoreapi.com/${environment}`
        }
        this.svc = new svc.ServiceClient(this)
    }
}

export namespace svc {
    export type Foo = number

    export interface GetRequest {
        Bar: string
        Baz: string
    }

    export interface Request {
        Foo?: Foo
        boo: string
        Raw: JSONValue
    }

    export interface Tuple<A, B> {
        A: A
        B: B
    }

    export type WrappedRequest = Wrapper<Request>

    export type Wrapper<T> = T

    export class ServiceClient {
        private client: Client

        constructor(client: Client) {
            this.client = client
        }

        public DummyAPI(params: Request) {
            return this.client.doVoid("POST", `/svc.DummyAPI`, params)
        }

        public Get(params: GetRequest) {
            const query: any[] = [
                "boo", params.Baz,
            ]
            return this.client.doVoid("GET", `/svc.Get?${encodeQuery(query)}`)
        }

        public RESTPath(a: string, b: number) {
            return this.client.doVoid("GET", `/path/${a}/${b}`)
        }

        public TupleInputOutput(params: Tuple<string, WrappedRequest>) {
            return this.client.do<Tuple<boolean, Foo>>("POST", `/svc.TupleInputOutput`, params)
        }
    }
}

// JSONValue represents an arbitrary JSON value.
export type JSONValue = string | number | boolean | null | JSONValue[] | { [key: string]: JSONValue }

function encodeQuery(parts: any[]): string {
    const pairs = []
    for (let i = 0; i < parts.length; i += 2) {
        const key = parts[i]
        let val = parts[i + 1]
        if (!Array.isArray(val)) {
            val = [val]
        }
        for (const v of val) {
            pairs.push(`${key}=${encodeURIComponent(v)}`)
        }
    }
    return pairs.join("&")
}
