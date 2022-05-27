import {Listbox, Menu, Transition} from "@headlessui/react";
import CodeMirror, {EditorConfiguration} from "codemirror";
import React, {FC, useEffect, useImperativeHandle, useRef, useState} from "react";
import HJSON from "hjson"
import * as icons from "~c/icons";
import Input from "~c/Input";
import {decodeBase64, encodeBase64} from "~lib/base64";
import JSONRPCConn from "~lib/client/jsonrpc";
import {copyToClipboard} from "~lib/clipboard";
import {APIMeta, PathSegment, RPC, Service} from "./api";
import CM from "./cm/CM";
import {FieldLocation, fieldNameAndLocation, Type, Builtin, NamedType} from "./schema";
import {JSONDialect} from "~c/api/SchemaView";

interface Props {
    conn: JSONRPCConn;
    appID: string;
    md: APIMeta;
    svc: Service;
    rpc: RPC;
    addr?: string;
}

export const cfg: EditorConfiguration = {
    theme:             "encore",
    mode:              "json",
    lineNumbers:       false,
    lineWrapping:      false,
    indentWithTabs:    true,
    indentUnit:        4,
    tabSize:           4,
    autoCloseBrackets: true,
    matchBrackets:     true,
    styleActiveLine:   false,
}

const APICallButton: FC<{ send: () => void; copyCurl: () => void; }> = (props) => {
    return (
        <span className="ml-auto flex-none relative z-0 inline-flex shadow-sm rounded-md self-start">
      <button type="button"
              className="relative inline-flex items-center px-4 py-2 rounded-l-md border border-purple-700 bg-purple-600 text-sm font-medium text-white hover:bg-purple-500 focus:z-10 focus:outline-none focus:ring-0 focus:border-purple-500"
              onClick={() => props.send()}>
        Call API
      </button>
      <span className="-ml-px relative block z-10">
        <Menu>
          {({open}) => (
              <>
                  <Menu.Button
                      className="relative inline-flex items-center px-2 py-2 rounded-r-md border border-purple-700 bg-purple-600 text-sm font-medium text-white hover:bg-purple-500 focus:z-10 focus:outline-none focus:ring-0">
                      <span className="sr-only">Open options</span>
                      {icons.chevronDown("h-5 w-5")}
                  </Menu.Button>

                  <Transition
                      show={open}
                      enter="transition ease-out duration-100"
                      enterFrom="transform opacity-0 scale-95"
                      enterTo="transform opacity-100 scale-100"
                      leave="transition ease-in duration-75"
                      leaveFrom="transform opacity-100 scale-100"
                      leaveTo="transform opacity-0 scale-95"
                  >
                      <Menu.Items
                          static
                          className="absolute right-0 w-56 mt-2 origin-top-right bg-white border border-gray-200 divide-y divide-gray-100 rounded-md shadow-lg outline-none"
                      >
                          <div className="py-1">
                              <Menu.Item>
                                  {({active}) => (
                                      <button
                                          className={`${
                                              active
                                                  ? "bg-gray-100 text-gray-900"
                                                  : "text-gray-700"
                                          } flex justify-between w-full px-4 py-2 text-sm leading-5 text-left`}
                                          onClick={() => props.copyCurl()}
                                      >
                                          Copy as curl
                                      </button>
                                  )}
                              </Menu.Item>
                          </div>
                      </Menu.Items>
                  </Transition>
              </>
          )}
        </Menu>
      </span>
    </span>
    )
}

const RPCCaller: FC<Props> = ({md, svc, rpc, conn, appID, addr}) => {
    const payloadCM = useRef<CM>(null)
    const authCM = useRef<CM>(null)
    const pathRef = useRef<{ getPath: () => string | undefined; getMethod: () => string }>(null)
    const docs = useRef(new Map<RPC, CodeMirror.Doc>())
    const authDoc = useRef<CodeMirror.Doc>(new CodeMirror.Doc("", {
        name: "javascript",
        json: true
    }))
    const authGeneratedJS = useRef("")
    const [authToken, setAuthToken] = useState("")
    const hasPathParams = rpc.path.segments.findIndex(s => s.type !== "LITERAL") !== -1

    const [loading, setLoading] = useState(false)
    const [respErr, setRespErr] = useState<string | undefined>(undefined)
    const [response, setResponse] = useState<string | undefined>(undefined)
    const [method, setMethod] = useState<string>(rpc.http_methods[0])

    const serializeRequest = (): [string, string, string] => {
        let path = pathRef.current?.getPath() ?? `/${svc.name}.${rpc.name}`
        let body = ''

        if (rpc.request_schema) {
            const doc = docs.current.get(rpc)
            if (doc === undefined) {
                return ["", "", ""]
            }
            body = doc.getValue()
        }

        return [path, body, authDoc.current.getValue()]
    }

    const makeRequest = async () => {
        const [path, reqBody, authBody] = serializeRequest()
        if (path === "") {
            return
        }
        try {
            setLoading(true)
            setResponse(undefined)
            setRespErr(undefined)
            const resp = await conn.request("api-call", {
                appID,
                service:      svc.name,
                endpoint:     rpc.name,
                method,
                path,
                auth_payload: encodeBase64(authBody),
                auth_token:   authToken,
                payload:      encodeBase64(reqBody)
            }) as any
            let respBody = ""
            if (resp.body.length > 0) {
                respBody = decodeBase64(resp.body)
            }

            if (resp.status_code !== 200) {
                setRespErr(`HTTP ${resp.status}: ${respBody}`)
            } else if (rpc.response_schema) {
                setResponse(respBody)
            } else {
                setResponse("Request completed successfully.")
            }
        } catch (err) {
            setRespErr(`Internal Error: ${err}`)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        if (method !== rpc.http_methods[0]) {
            setMethod(rpc.http_methods[0])
        }
    }, [rpc])

    function namedTypeToHJSON(named: NamedType): string {
        const render = new JSONDialect(md)
        render.method = method
        render.asResponse = false
        render.typeArgumentStack.push(named.type_arguments)
        const [queryString, headers, js] = render.structBits(md.decls[named.id].type.struct!, true)

        let bits: string[] = ["{\n"]
        let previousSection = false
        if (headers) {
            bits.push("    // HTTP headers", headers)
            previousSection = true
        }
        if (queryString) {
            if (previousSection) {
                bits.push(",\n\n")
            }

            bits.push("    // Query string", queryString)
            previousSection = true
        }
        if (js) {
            if (previousSection) {
                bits.push(",\n\n")
            }

            bits.push("    // HTTP body", js)
        }
        bits.push("\n}")

        return bits.join("")
    }

    useEffect(() => {
        if (rpc.request_schema) {
            let doc = new CodeMirror.Doc(namedTypeToHJSON(rpc.request_schema.named!), {
                name: "javascript",
                json: true
            })
            docs.current.set(rpc, doc)
            payloadCM.current?.open(doc)
        }

        setResponse(undefined)
        setRespErr(undefined)
    }, [rpc, method])

    useEffect(() => {
        if (md.auth_handler?.params?.named) {
            const generated = namedTypeToHJSON(md.auth_handler.params.named)

            if (authGeneratedJS.current !== generated) {
                authDoc.current.setValue("// Authentication Data\n" + generated)
                authCM.current?.open(authDoc.current)
                authGeneratedJS.current = generated
            }
        }
    })

    const copyCurl = () => {
        let [path, reqBody, authBody] = serializeRequest()
        if (path === "") {
            return
        }

        let headers: Record<string, any> = {}
        let queryString = ''
        function addQuery(name: string, value: any) {
            if (Array.isArray(value)) {
                return value.map((v) => {
                    addQuery(name, v)
                })
            }

            if (queryString) {
                queryString += '&'
            } else {
                queryString = '?'
            }
            queryString += name + '=' + encodeURIComponent(value)
        }

        const newBody: Record<string, any> = {}
        function processStruct(named: NamedType, payload: string) {
            try {
                const astFields = md.decls[named.id].type.struct!.fields

                const bodyFields: Record<string, any> = HJSON.parse(payload)
                if (typeof (bodyFields) !== "object") {
                    throw new Error("Request Body isn't a JSON object")
                }

                for (const fieldName in bodyFields) {
                    if (!bodyFields.hasOwnProperty(fieldName)) {
                        continue
                    }

                    const fieldValue = bodyFields[fieldName]

                    for (const f of astFields) {
                        if (f.name === fieldName) {
                            let [encodedName, location] = fieldNameAndLocation(f, method, false)

                            switch (location) {
                                case FieldLocation.Header:
                                    headers[encodedName] = fieldValue
                                    break
                                case FieldLocation.Query:
                                    addQuery(encodedName, fieldValue)
                                    break
                                case FieldLocation.Body:
                                    newBody[encodedName] = fieldValue
                                    break
                            }
                        }
                    }
                }

            } catch (e) {
                console.error("Unable to parse body: ", e)
                // but continue anyway
            }
        }

        if (rpc.request_schema?.named) {
            processStruct(rpc.request_schema.named, reqBody)
        }
        if (md.auth_handler?.params?.named) {
            processStruct(md.auth_handler.params.named, authBody)
        }

        reqBody = JSON.stringify(newBody)

        const defaultMethod = (reqBody !== "" ? "POST" : "GET")
        let cmd = "curl "
        if (method !== defaultMethod) {
            cmd += `-X ${method} `
        }
        cmd += `'http://${addr ?? "localhost:4000"}${path}${queryString}'`

        for (const header in headers) {
            cmd += ` -H "${header}: ${headers[header]}"`
        }

        if (reqBody !== "") {
            cmd += ` -d '${reqBody}'`
        }
        copyToClipboard(cmd)
    }

    return (
        <div>
            <h4 className="text-base text-bold">
                Request
            </h4>
            <div
                className={`text-xs mt-1 rounded border border-gray-200 ${rpc.request_schema || hasPathParams ? "block" : "hidden"} p-1 divide-y divide-gray-500`}
                style={{backgroundColor: "#2d3748"}}>
                <div>
                    <RPCPathEditor ref={pathRef} rpc={rpc} method={method} setMethod={setMethod}/>
                </div>
                <div className={`${rpc.request_schema ? "block" : " hidden"}`}>
                    <CM ref={payloadCM} cfg={cfg}/>
                </div>
            </div>
            <div className={`text-xs mt-1 ${rpc.request_schema ? "hidden" : "block"}`}>
                This API takes no request data.
            </div>
            <div className="flex items-center mt-1 items-start">
                {md.auth_handler && md.auth_handler.params?.builtin === Builtin.STRING &&
                    <div className="flex-1 min-w-0 mr-1 relative rounded-md shadow-sm">
                        <Input id="" cls="w-full" placeholder="Auth Token" required={rpc.access_type === "AUTH"}
                               value={authToken} onChange={setAuthToken}/>
                    </div>
                }
                {md.auth_handler && md.auth_handler.params?.named !== undefined &&
                    <div className="text-xs flex-1 min-w-0 mr-1 relative rounded-md shadow-sm">
                        <CM ref={authCM} cfg={cfg}/>
                    </div>
                }
                <APICallButton send={makeRequest} copyCurl={copyCurl}/>

            </div>

            <h4 className="mt-4 mb-1 text-base text-bold flex items-center">
                Response {loading && icons.loading("ml-1 h-5 w-5", "#A081D9", "transparent", 4)}
            </h4>
            {response ? (
                <pre
                    className="text-xs shadow-inner rounded border border-gray-300 bg-gray-200 p-2 overflow-x-auto response-docs">
          <CM
              key={response}
              cfg={{
                  value:    response,
                  readOnly: true,
                  theme:    "encore",
                  mode:     {name: "javascript", json: true},
              }}
              noShadow={true}
          />
        </pre>
            ) : respErr ? (
                <div className="text-xs text-red-600 font-mono">{respErr}</div>
            ) : (
                <div className="text-xs text-gray-400">Make a request to see the response.</div>
            )}
        </div>
    )
}

export default RPCCaller

export const pathEditorCfg: EditorConfiguration = {
    theme:             "encore",
    mode:              "json",
    lineNumbers:       false,
    lineWrapping:      false,
    indentWithTabs:    true,
    indentUnit:        4,
    tabSize:           4,
    autoCloseBrackets: true,
    matchBrackets:     true,
    styleActiveLine:   false,
    extraKeys:         {
        Tab:         (cm: CodeMirror.Editor) => {
            const doc = cm.getDoc()
            const cur = doc.getCursor()
            if (!cur) {
                return
            }
            const markers = (doc.getAllMarks() as CodeMirror.TextMarker<CodeMirror.MarkerRange>[]).filter(m => !m.readOnly).map(m => m.find()).filter(m => m !== undefined).sort((a, b) => {
                return a!.from.ch - b!.from.ch
            })

            for (let i = 0; i < markers.length; i++) {
                const m = markers[i]
                if (m!.from.ch <= cur.ch && m!.to.ch >= cur.ch) {
                    if ((i + 1) < markers.length) {
                        const m2 = markers[i + 1]
                        doc.setSelection(m2!.from, m2!.to)
                    } else if (i > 0) {
                        const m2 = markers[0]
                        doc.setSelection(m2!.from, m2!.to)
                    }
                    return
                }
            }
        },
        "Shift-Tab": (cm: CodeMirror.Editor) => {
            const doc = cm.getDoc()
            const cur = doc.getCursor()
            if (!cur) {
                return
            }
            const markers = (doc.getAllMarks() as CodeMirror.TextMarker<CodeMirror.MarkerRange>[]).filter(m => !m.readOnly).map(m => m.find()).filter(m => m !== undefined).sort((a, b) => {
                return a!.from.ch - b!.from.ch
            })

            for (let i = 0; i < markers.length; i++) {
                const m = markers[i]
                if (m!.from.ch <= cur.ch && m!.to.ch >= cur.ch) {
                    if ((i - 1) >= 0) {
                        const m2 = markers[i - 1]
                        doc.setSelection(m2!.from, m2!.to)
                    } else if (markers.length > 1) {
                        const m2 = markers[markers.length - 1]
                        doc.setSelection(m2!.from, m2!.to)
                    }
                    return
                }
            }
        }
    }
}

function classNames(...classes: string[]) {
    return classes.filter(Boolean).join(' ')
}

const RPCPathEditor = React.forwardRef<{ getPath: () => string | undefined }, {
    rpc: RPC; method: string; setMethod: (m: string) => void;
}>(({rpc, method, setMethod}, ref) => {
    interface DocState {
        rpc: RPC;
        doc: CodeMirror.Doc;
        markers: CodeMirror.TextMarker<CodeMirror.MarkerRange>[];
    }

    const pathCM = useRef<CM>(null)
    const docs = useRef(new Map<RPC, DocState>())
    const docMap = useRef(new Map<CodeMirror.Doc, DocState>())
    const timeoutHandle = useRef<{ id: number | null }>({id: null})

    // Reset the method when the RPC changes
    useEffect(() => {
        setMethod(rpc.http_methods[0])
    }, [rpc])

    useEffect(() => {
        const segments: string[] = []

        type rwSegment = { from: number; to: number; placeholder: string; seg: PathSegment };
        const readWrites: rwSegment[] = []
        let pos = 0
        for (const s of rpc.path.segments) {
            segments.push("/")
            pos += 1

            const placeholder = (s.type === "PARAM" ? ":" : s.type === "WILDCARD" ? "*" : "") + s.value
            const ln = placeholder.length
            segments.push(placeholder)
            if (s.type !== "LITERAL") {
                readWrites.push({placeholder, seg: s, from: pos, to: pos + ln})
            }
            pos += ln
        }

        const val = segments.join("")
        const doc = new CodeMirror.Doc(val,)

        let prevEnd = 0
        let i = 0
        const markers: CodeMirror.TextMarker<CodeMirror.MarkerRange>[] = []
        for (const rw of readWrites) {
            doc.markText({ch: prevEnd, line: 0}, {ch: rw.from, line: 0}, {
                atomic:         true,
                readOnly:       true,
                clearWhenEmpty: false,
                clearOnEnter:   false,
                className:      "text-gray-400",
                selectLeft:     i > 0,
                selectRight:    true,
            })
            const m = doc.markText({ch: rw.from, line: 0}, {ch: rw.to, line: 0}, {
                className:      "text-green-400",
                clearWhenEmpty: false,
                clearOnEnter:   false,
                inclusiveLeft:  true,
                inclusiveRight: true,
                attributes:     {placeholder: rw.placeholder, segmentType: rw.seg.type},
            })
            markers.push(m)
            m.on("beforeCursorEnter", () => {
                const r = m.find()
                const sel = doc.getSelection()
                if (r) {
                    const text = doc.getRange(r.from, r.to)
                    if (text === m.attributes?.placeholder && sel !== text) {
                        if (timeoutHandle.current.id) {
                            clearTimeout(timeoutHandle.current.id)
                        }
                        timeoutHandle.current.id = setTimeout(() => {
                            doc.setSelection(r.from, r.to)
                        }, 50)
                    }
                }
            })
            prevEnd = rw.to
            i++
        }

        doc.markText({ch: prevEnd, line: 0}, {ch: val.length, line: 0}, {
            atomic:         true,
            readOnly:       true,
            clearWhenEmpty: false,
            clearOnEnter:   false,
            className:      "text-gray-400",
            selectLeft:     prevEnd > 0,
            selectRight:    false,
        })

        CodeMirror.on(doc, "beforeChange", (doc: CodeMirror.Doc, change: CodeMirror.EditorChangeCancellable) => {
            if (change.text[0].indexOf("/") === -1) {
                return
            }

            for (const m of markers) {
                const r = m.find()
                if (r && change.from.ch >= r.from.ch && change.from.ch <= r.to.ch) {
                    const typ = m.attributes?.segmentType
                    if (typ === "PARAM") {
                        change.cancel()
                    }
                    return
                }
            }
        })

        const ds = {rpc, doc, markers: markers}
        docs.current.set(rpc, ds)
        docMap.current.set(doc, ds)
        pathCM.current?.open(ds!.doc)
    }, [rpc, method])

    useImperativeHandle(ref, () => {
        // noinspection JSUnusedGlobalSymbols
        return {
            getPath:   () => pathCM.current?.cm?.getValue(),
            getMethod: () => method,
        }
    })

    return <div className="flex items-center">
        {rpc.http_methods.length > 1 ? (
            <Listbox value={method} onChange={setMethod}>
                {({open}) => (
                    <div className="relative">
                        <Listbox.Button
                            className="relative block text-left cursor-default focus:outline-none pl-1 pr-5 py-0.5 text-green-800 bg-green-100 hover:bg-green-200 rounded-sm font-mono font-semibold text-xs">
                            <span className="block truncate">{method}</span>
                            <span className="absolute inset-y-0 right-0 flex items-center pointer-events-none">
                {icons.chevronDown("h-5 w-5 text-green-600")}
              </span>
                        </Listbox.Button>
                        <Transition
                            show={open}
                            leave="transition ease-in duration-100"
                            leaveFrom="opacity-100"
                            leaveTo="opacity-0"
                        >
                            <Listbox.Options
                                static
                                className="absolute z-10 mt-1 w-32 bg-white shadow-lg max-h-60 rounded py-1 ring-1 ring-black ring-opacity-5 overflow-auto focus:outline-none text-xs"
                            >
                                {rpc.http_methods.map((m) => (
                                    <Listbox.Option
                                        key={m}
                                        className={({active}) =>
                                            classNames(
                                                active ? 'text-white bg-green-600' : 'text-gray-900',
                                                'cursor-default select-none relative py-1 pl-3 pr-9'
                                            )
                                        }
                                        value={m}
                                    >
                                        {({selected, active}) => (
                                            <>
                        <span className={classNames(selected ? 'font-semibold' : 'font-normal', 'block truncate')}>
                          {m}
                        </span>

                                                {selected ? (
                                                    <span
                                                        className={classNames(
                                                            active ? 'text-white' : 'text-green-600',
                                                            'absolute inset-y-0 right-0 flex items-center pr-4'
                                                        )}
                                                    >
                            {icons.check("h-5 w-5")}
                          </span>
                                                ) : null}
                                            </>
                                        )}
                                    </Listbox.Option>
                                ))}
                            </Listbox.Options>
                        </Transition>
                    </div>
                )}
            </Listbox>
        ) : <div className="text-white font-mono text-xs px-1">{method}</div>}
        <div className="flex-1">
            <CM ref={pathCM} cfg={pathEditorCfg} className="overflow-visible"/>
        </div>
    </div>
})

// encodeQuery encodes a payload matching the given schema as a query string.
// If the payload can't be parsed as JSON it throws an exception.
function encodeQuery(md: APIMeta, schema: Type, payload: string): string {
    const json = JSON.parse(payload)
    let pairs: string[] = []

    if (schema.named) {
        const declID = schema.named.id
        const decl = md.decls[declID]

        for (const f of decl.type.struct?.fields ?? []) {
            let key = f.json_name
            let qsName = f.query_string_name
            if (key === "-" || qsName === "-") {
                continue
            } else if (key === "") {
                key = f.name
            }

            let val = json[key]
            if (typeof val === "undefined") {
                continue
            } else if (!Array.isArray(val)) {
                val = [val]
            }
            for (const v of val) {
                pairs.push(`${qsName}=${encodeURIComponent(v)}`)
            }
        }
    } else {
        throw new Error('expected a named type to encode the query');
    }

    return pairs.join("&")
}
