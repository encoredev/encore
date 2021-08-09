package codegen

import (
	"strings"
	"testing"

	"encr.dev/parser"
	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"
)

func TestTypeScript(t *testing.T) {
	c := qt.New(t)

	const code = `
-- go.mod --
module app

-- encore.app --
{"id": ""}

-- svc/svc.go --
package svc

type Request struct {
    Foo Foo
}

type Foo int

-- svc/api.go --
package svc

import "context"

//encore:api public
func DummyAPI(ctx context.Context, req *Request) error {
    return nil
}
`

	ar := txtar.Parse([]byte(code))
	base := t.TempDir()
	err := txtar.Write(ar, base)
	c.Assert(err, qt.IsNil)

	res, err := parser.Parse(&parser.Config{
		AppRoot:    base,
		ModulePath: "app",
	})
	c.Assert(err, qt.IsNil)

	ts, err := Client(TypeScript, "app", res.Meta)
	c.Assert(err, qt.IsNil)
	expect := `export default class Client {
    svc: svc.ServiceClient

    constructor(environment: string = "prod", token?: string) {
        const base = new BaseClient(environment, token)
        this.svc = new svc.ServiceClient(base)
    }
}

export namespace svc {
    export type Foo = number

    export interface Request {
        Foo: Foo
    }

    export class ServiceClient {
        private baseClient: BaseClient

        constructor(baseClient: BaseClient) {
            this.baseClient = baseClient
        }

        public DummyAPI(params: Request): Promise<void> {
            return this.baseClient.doVoid("POST", ` + "`/svc.DummyAPI`" + `, params)
        }
    }
}

class BaseClient {
    baseURL: string
    headers: {[key: string]: string}

    constructor(environment: string, token?: string) {
        this.headers = {"Content-Type": "application/json"}
        if (token !== undefined) {
            this.headers["Authorization"] = "Bearer " + token
        }
        if (environment === "local") {
            this.baseURL = "http://localhost:4060"
        } else {
            this.baseURL = ` + "`" + `https://app.encoreapi.com/${environment}` + "`" + `
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
`

	c.Assert(strings.Split(string(ts), "\n"), qt.DeepEquals, strings.Split(expect, "\n"))
}
