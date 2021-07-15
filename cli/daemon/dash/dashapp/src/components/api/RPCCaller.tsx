import { Listbox, Menu, Transition } from "@headlessui/react";
import CodeMirror, { EditorConfiguration } from "codemirror";
import React, { FC, useEffect, useImperativeHandle, useRef, useState } from "react";
import * as icons from "~c/icons";
import Input from "~c/Input";
import { decodeBase64, encodeBase64 } from "~lib/base64";
import JSONRPCConn from "~lib/client/jsonrpc";
import { copyToClipboard } from "~lib/clipboard";
import { APIMeta, PathSegment, RPC, Service } from "./api";
import CM from "./cm/CM";
import { BuiltinType, Decl, ListType, MapType, NamedType, StructType, Type } from "./schema";

interface Props {
  conn: JSONRPCConn;
  appID: string;
  md: APIMeta;
  svc: Service;
  rpc: RPC;
  port?: number;
}

interface State {
  loading: boolean;
  response?: string;
  respErr?: string;
  authToken: string;
}

export const cfg: EditorConfiguration = {
  theme: "encore",
  mode: "json",
  lineNumbers: false,
  lineWrapping: false,
  indentWithTabs: true,
  indentUnit: 4,
  tabSize: 4,
  autoCloseBrackets: true,
  matchBrackets: true,
  styleActiveLine: false,
}

class JSONRenderer {
  buf: string[];
  level: number;
  md: APIMeta;
  seenDecls: Set<number>;

  constructor(md: APIMeta) {
    this.buf = []
    this.level = 0
    this.md = md
    this.seenDecls = new Set()
  }

  render(d: Decl): string {
    this.writeType(d.type)
    return this.buf.join("")
  }

  private writeType(t: Type) {
    t.struct ? this.renderStruct(t.struct) :
    t.map ? this.renderMap(t.map) :
    t.list ? this.renderList(t.list) :
    t.builtin ? this.write(this.renderBuiltin(t.builtin)) :
    t.named ? this.renderNamed(t.named)
    : this.write("<unknown type>")
  }

  private renderNamed(t: NamedType) {
    if (this.seenDecls.has(t.id)) {
      this.write("null")
      return
    }
    
    // Add the decl to our map while recursing to avoid infinite recursion.
    this.seenDecls.add(t.id)
    const decl = this.md.decls[t.id]
    this.writeType(decl.type)
    this.seenDecls.delete(t.id)
  }

  private renderStruct(t: StructType) {
    this.writeln("{")
    this.level++
    for (let i = 0; i < t.fields.length; i++) {
      const f = t.fields[i]
      this.indent()
      this.write(`"${f.json_name !== "" ? f.json_name : f.name}": `)
      this.writeType(f.typ)
      if (i < (t.fields.length-1)) {
        this.write(",")
      }
      this.writeln()
    }
    this.level--
    this.indent()
    this.write("}")
  }

  private renderMap(t: MapType) {
    this.writeln("{")
    this.level++
    this.indent()
    this.writeType(t.key)
    this.write(": ")
    this.writeType(t.value)
    this.writeln()
    this.write("}")
  }

  private renderList(t: ListType) {
    this.write("[")
    this.writeType(t.elem)
    this.write("]")
  }

  private renderBuiltin(t: BuiltinType) {
    switch (t) {
    case BuiltinType.Any: return "<any>"
    case BuiltinType.Bool: return "true"
    case BuiltinType.Int: return "1"
    case BuiltinType.Int8: return "1"
    case BuiltinType.Int16: return "1"
    case BuiltinType.Int32: return "1"
    case BuiltinType.Int64: return "1"
    case BuiltinType.Uint: return "1"
    case BuiltinType.Uint8: return "1"
    case BuiltinType.Uint16: return "1"
    case BuiltinType.Uint32: return "1"
    case BuiltinType.Uint64: return "1"
    case BuiltinType.Float32: return "2.3"
    case BuiltinType.Float64: return "2.3"
    case BuiltinType.String: return "\"some string\""
    case BuiltinType.Bytes: return "\"base64-encoded-bytes\""
    case BuiltinType.Time: return "\"2009-11-10T23:00:00Z\""
    case BuiltinType.UUID: return "\"7d42f515-3517-4e76-be13-30880443546f\""
    case BuiltinType.JSON: return "{\"some json data\": true}"
    default: return "<unknown>"
    }
  }

  private indent() {
    this.write(" ".repeat(this.level*4))
  }

  private write(...strs: string[]) {
    for (const s of strs) {
      this.buf.push(s)
    }
  }

  private writeln(...strs: string[]) {
    this.write(...strs)
    this.write("\n")
  }
}

const APICallButton: FC<{send: () => void; copyCurl: () => void;}> = (props) => {
  return (
    <span className="ml-auto flex-none relative z-0 inline-flex shadow-sm rounded-md">
      <button type="button" className="relative inline-flex items-center px-4 py-2 rounded-l-md border border-purple-700 bg-purple-600 text-sm font-medium text-white hover:bg-purple-500 focus:z-10 focus:outline-none focus:ring-0 focus:border-purple-500"
        onClick={() => props.send()}>
        Call API
      </button>
      <span className="-ml-px relative block z-10">
        <Menu>
          {({ open }) => (
            <>
              <Menu.Button className="relative inline-flex items-center px-2 py-2 rounded-r-md border border-purple-700 bg-purple-600 text-sm font-medium text-white hover:bg-purple-500 focus:z-10 focus:outline-none focus:ring-0">
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
                      {({ active }) => (
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

const RPCCaller: FC<Props> = ({md, svc, rpc, conn, appID, port}) => {
  const payloadCM = useRef<CM>(null)
  const pathRef = useRef<{getPath: () => string | undefined; getMethod: () => string}>(null)
  const docs = useRef(new Map<RPC, CodeMirror.Doc>())
  const [authToken, setAuthToken] = useState("")
  const hasPathParams = rpc.path.segments.findIndex(s => s.type !== "LITERAL") !== -1

  const [loading, setLoading] = useState(false)
  const [respErr, setRespErr] = useState<string | undefined>(undefined)
  const [response, setResponse] = useState<string | undefined>(undefined)

  const makeRequest = async () => {
    let reqBody = ""
    if (rpc.request_schema) {
      const doc = docs.current.get(rpc)
      if (doc === undefined) {
        return
      }
      reqBody = doc.getValue()
    }

    const payload = encodeBase64(reqBody)
    const method = pathRef.current?.getMethod() ?? "POST"
    const path = pathRef.current?.getPath() ?? `/${svc.name}.${rpc.name}`
    try {
      setLoading(true)
      setRespErr(undefined)
      const resp = await conn.request("api-call", {appID, method, path, payload, authToken}) as any
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
    } catch(err) {
      setRespErr(`Internal Error: ${err}`)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (rpc.request_schema) {
      let doc = docs.current.get(rpc)
      if (doc === undefined) {
        const js = new JSONRenderer(md).render(rpc.request_schema!)
        doc = new CodeMirror.Doc(js, {
          name: "javascript",
          json: true
        })
        docs.current.set(rpc, doc)
      }
      payloadCM.current?.open(doc)
    }

    setResponse(undefined)
    setRespErr(undefined)
  }, [rpc])


  const copyCurl = () => {
    let reqBody = ""
    if (rpc.request_schema) {
      const doc = docs.current.get(rpc)
      if (doc === undefined) {
        return
      }
      reqBody = doc.getValue()
      // Convert to JSON and back, if possible, to simplify indentation
      try {
        reqBody = JSON.stringify(JSON.parse(reqBody), undefined, " ")
      } catch(err) { /* do nothing */ }

      reqBody = reqBody.replaceAll("'", "'\''") // escape single quotes
    }
    const path = pathRef.current?.getPath() ?? `/${svc.name}.${rpc.name}`

    const method = pathRef.current?.getMethod() ?? "POST"
    const defaultMethod = (reqBody !== "" ? "POST" : "GET")

    let cmd = "curl "
    if (method !== defaultMethod) {
      cmd += `-X ${method} `
    }
    cmd += `http://localhost:${port ?? 4060}${path}`
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
      <div className={`text-xs mt-1 rounded border border-gray-200 ${rpc.request_schema || hasPathParams ? "block" : "hidden"} p-1 divide-y divide-gray-500`} style={{backgroundColor: "#2d3748"}}>
        <div className={`${hasPathParams ? "block" : " hidden"}`}>
          <RPCPathEditor ref={pathRef} svc={svc} rpc={rpc} />
         </div>
        <div className={`${rpc.request_schema ? "block" : " hidden"}`}>
          <CM ref={payloadCM} cfg={cfg} />
         </div>
      </div>
      <div className={`text-xs mt-1 ${rpc.request_schema ? "hidden" : "block"}`}>
        This API takes no request data.
      </div>
      <div className="flex items-center mt-1">
        {md.auth_handler && 
          <div className="flex-1 min-w-0 mr-1 relative rounded-md shadow-sm">
            <Input id="" cls="w-full" placeholder="Auth Token" required={rpc.access_type === "AUTH"} 
              value={authToken} onChange={setAuthToken} />
          </div>
        }
        <APICallButton send={makeRequest} copyCurl={copyCurl} />
        
      </div>

      <h4 className="mt-4 mb-1 text-base text-bold flex items-center">
        Response {loading && icons.loading("ml-1 h-5 w-5", "#A081D9", "transparent", 4)}
      </h4>
      {response ? (
        <pre className="text-xs shadow-inner rounded border border-gray-300 bg-gray-200 p-2 overflow-x-auto">{response}</pre>
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
  theme: "encore",
  mode: "json",
  lineNumbers: false,
  lineWrapping: false,
  indentWithTabs: true,
  indentUnit: 4,
  tabSize: 4,
  autoCloseBrackets: true,
  matchBrackets: true,
  styleActiveLine: false,
  extraKeys: {
    Tab: (cm: CodeMirror.Editor) => {
      const doc = cm.getDoc()
      const cur = doc.getCursor()
      if (!cur) { return }
      const markers = (doc.getAllMarks() as CodeMirror.TextMarker<CodeMirror.MarkerRange>[]).
        filter(m => !m.readOnly).map(m => m.find()).filter(m => m !== undefined).sort((a, b) => { return a!.from.ch - b!.from.ch})

      for (let i = 0; i < markers.length; i++) {
        const m = markers[i]
        if (m!.from.ch <= cur.ch && m!.to.ch >= cur.ch) {
          if ((i+1) < markers.length) {
            const m2 = markers[i+1]
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
      if (!cur) { return }
      const markers = (doc.getAllMarks() as CodeMirror.TextMarker<CodeMirror.MarkerRange>[]).
        filter(m => !m.readOnly).map(m => m.find()).filter(m => m !== undefined).sort((a, b) => { return a!.from.ch - b!.from.ch})

      for (let i = 0; i < markers.length; i++) {
        const m = markers[i]
        if (m!.from.ch <= cur.ch && m!.to.ch >= cur.ch) {
          if ((i-1) >= 0) {
            const m2 = markers[i-1]
            doc.setSelection(m2!.from, m2!.to)
          } else if (markers.length > 1) {
            const m2 = markers[markers.length-1]
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

const RPCPathEditor = React.forwardRef<{getPath: () => string | undefined; getMethod: () => string}, {svc: Service; rpc: RPC}>(({svc, rpc}, ref) => {
  interface DocState {
    rpc: RPC;
    doc: CodeMirror.Doc;
    markers: CodeMirror.TextMarker<CodeMirror.MarkerRange>[];
  }
  const pathCM = useRef<CM>(null)
  const docs = useRef(new Map<RPC, DocState>())
  const docMap = useRef(new Map<CodeMirror.Doc, DocState>())
  const timeoutHandle = useRef<{id: number | null}>({id: null})
  const [method, setMethod] = useState(rpc.http_methods[0])

  useEffect(() => {
    let ds = docs.current.get(rpc)
    if (ds === undefined) {
      const segments: string[] = []
      const readWrites: {from: number, to: number; placeholder: string; seg: PathSegment}[] = []
      let pos = 0
      for (const s of rpc.path.segments) {
        segments.push("/")
        pos += 1

        const placeholder = (s.type === "PARAM" ? ":" : s.type === "WILDCARD" ? "*" : "") + s.value
        const ln = placeholder.length
        segments.push(placeholder)
        if (s.type !== "LITERAL") {
          readWrites.push({placeholder, seg: s, from: pos, to: pos+ln})
        }
        pos += ln
      }

      const val = segments.join("")
      const doc = new CodeMirror.Doc(val)

      let prevEnd = 0
      let i = 0
      const markers: CodeMirror.TextMarker<CodeMirror.MarkerRange>[] = []
      for (const rw of readWrites) {
        doc.markText({ch: prevEnd, line: 0}, {ch: rw.from, line: 0}, {
          atomic: true,
          readOnly: true,
          clearWhenEmpty: false,
          clearOnEnter: false,
          className: "text-gray-400",
          selectLeft: i>0,
          selectRight: true,
        })
        const m = doc.markText({ch: rw.from, line: 0}, {ch: rw.to, line: 0}, {
          className: "text-green-400",
          clearWhenEmpty: false,
          clearOnEnter: false,
          inclusiveLeft: true,
          inclusiveRight: true,
          attributes: {placeholder: rw.placeholder, segmentType: rw.seg.type},
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
              timeoutHandle.current.id = setTimeout(() => { doc.setSelection(r.from, r.to) }, 50)
            }
          }
        })
        prevEnd = rw.to
        i++
      }

      doc.markText({ch: prevEnd, line: 0}, {ch: val.length, line: 0}, {
        atomic: true,
        readOnly: true,
        clearWhenEmpty: false,
        clearOnEnter: false,
        className: "text-gray-400",
        selectLeft: true,
        selectRight: false,
      })
      
      CodeMirror.on(doc, "beforeChange", (doc: CodeMirror.Doc, change: CodeMirror.EditorChangeCancellable) => {
        if (change.text[0].indexOf("/") === -1) {
          return
        }

        for (const m of markers) {
          const r = m.find()
          if (r && change.from.ch >= r.from.ch && change.from.ch <= r.to.ch) {
            if (m.attributes?.segmentType === "PARAM") {
              change.cancel()
            }
            return
          }
        }
      })

      ds = {rpc, doc, markers: markers}
      docs.current.set(rpc, ds)
      docMap.current.set(doc, ds)
    }

    pathCM.current?.open(ds!.doc)
  }, [rpc])

  useImperativeHandle(ref, () => {
    return {
      getPath: () => pathCM.current?.cm?.getValue(),
      getMethod: () => method,
    }
  })

  return <div className="flex items-center">
    {rpc.http_methods.length > 1 ? (
      <Listbox value={method} onChange={setMethod}>
        {({ open }) => (
          <div className="relative">
            <Listbox.Button className="relative block text-left cursor-default focus:outline-none pl-1 pr-5 py-0.5 text-green-800 bg-green-100 hover:bg-green-200 rounded-sm font-mono font-semibold text-xs">
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
                    className={({ active }) =>
                      classNames(
                        active ? 'text-white bg-green-600' : 'text-gray-900',
                        'cursor-default select-none relative py-1 pl-3 pr-9'
                      )
                    }
                    value={m}
                  >
                    {({ selected, active }) => (
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
      <CM ref={pathCM} cfg={pathEditorCfg} className="overflow-visible" />
     </div>
  </div>
})