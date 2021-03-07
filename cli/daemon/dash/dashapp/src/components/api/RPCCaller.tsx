import { Type, StructType, MapType, ListType, BuiltinType, NamedType, Decl } from "./schema";
import React from "react";
import CM from "./cm/CM";
import { EditorConfiguration } from "codemirror";
import CodeMirror from "codemirror";
import Button from "~c/Button";
import * as icons from "~c/icons";
import { APIMeta, RPC, Service } from "./api";
import JSONRPCConn from "~lib/client/jsonrpc";
import { decodeBase64, encodeBase64 } from "~lib/base64";
import Input from "~c/Input";

interface Props {
  conn: JSONRPCConn;
  appID: string;
  md: APIMeta;
  svc: Service;
  rpc: RPC;
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

export default class RPCCaller extends React.Component<Props, State> {
  cm: React.RefObject<CM>;
  docs: Map<RPC, CodeMirror.Doc>;

  constructor(props: Props) {
    super(props)
    this.cm = React.createRef()
    this.docs = new Map()
    this.state = {loading: false, authToken: ""}
  }

  componentDidMount() {
    this.open(this.props.rpc)
  }

  private open(rpc: RPC) {
    if (rpc.request_schema) {
      let doc = this.docs.get(rpc)
      if (doc === undefined) {
        const js = new JSONRenderer(this.props.md).render(rpc.request_schema!)
        doc = new CodeMirror.Doc(js, {
          name: "javascript",
          json: true
        })
        this.docs.set(rpc, doc)
      }
      this.cm.current?.open(doc)
    }
    this.setState({response: undefined, respErr: undefined})
  }

  async send() {
    const rpc = this.props.rpc
    let reqBody = ""
    if (rpc.request_schema) {
      const doc = this.docs.get(rpc)
      if (doc === undefined) {
        return
      }
      reqBody = doc.getValue()
    }

    const payload = encodeBase64(reqBody)
    const authToken = this.state.authToken
    const endpoint = `${this.props.svc.name}.${rpc.name}`
    try {
      this.setState({loading: true})
      const resp = await this.props.conn.request("api-call", {
        appID: this.props.appID,
        endpoint,
        payload,
        authToken,
      }) as any

      let respBody = ""
      if (resp.body.length > 0) {
        respBody = decodeBase64(resp.body)
      }
      if (resp.status_code !== 200) {
        this.setState({response: undefined, respErr: `HTTP ${resp.status}: ${respBody}`})
      } else if (rpc.response_schema) {
        this.setState({response: respBody, respErr: undefined})
      } else {
        this.setState({response: "Request completed successfully.", respErr: undefined})
      }
    } catch(err) {
      this.setState({response: undefined, respErr: `Internal Error: ${err}`})
    } finally {
      this.setState({loading: false})
    }
  }

  render() {
    const rpc = this.props.rpc
    return (
      <div>
        <h4 className="text-base text-bold">
          Request
        </h4>
        <div className={`text-xs mt-1 rounded border border-gray-200 ${rpc.request_schema ? "block" : "hidden"}`}>
          <CM ref={this.cm} cfg={cfg} />
        </div>
        <div className={`text-xs mt-1 ${rpc.request_schema ? "hidden" : "block"}`}>
          This API takes no request data.
        </div>
        <div className="flex items-center mt-1">
          {this.props.md.auth_handler && 
            <div className="flex-1 min-w-0 mr-1 relative rounded-md shadow-sm">
              <Input id="" cls="w-full" placeholder="Auth Token" required={rpc.access_type === "AUTH"} 
                value={this.state.authToken} onChange={(authToken) => this.setState({authToken})} />
            </div>
          }
          
          <Button cls="ml-auto flex-none" size="md" theme="purple"
              onClick={() => this.send()}
            >Send {icons.chevronRight("h-4 w-auto ml-0.5 -mr-1.5")}</Button>
        </div>

        <h4 className="mt-4 mb-1 text-base text-bold flex items-center">
          Response {this.state.loading && icons.loading("ml-1 h-5 w-5", "#A081D9", "transparent", 4)}
        </h4>
        {this.state.response ? (
          <pre className="text-xs shadow-inner rounded border border-gray-300 bg-gray-200 p-2 overflow-x-auto">
            {this.state.response}
          </pre>
        ) : this.state.respErr ? (
          <div className="text-xs text-red-600 font-mono">
            {this.state.respErr}
          </div>
        ) : <div className="text-xs text-gray-400">Make a request to see the response.</div>}
      </div>
    )
  }
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
    case BuiltinType.Int8: return "1"
    case BuiltinType.Int16: return "1"
    case BuiltinType.Int32: return "1"
    case BuiltinType.Int64: return "1"
    case BuiltinType.Uint8: return "1"
    case BuiltinType.Uint16: return "1"
    case BuiltinType.Uint32: return "1"
    case BuiltinType.Uint64: return "1"
    case BuiltinType.Float32: return "2.3"
    case BuiltinType.Float64: return "fl2.3"
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