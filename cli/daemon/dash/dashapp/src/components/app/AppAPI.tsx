import React, {FunctionComponent, useState} from 'react'
import {APIMeta, RPC, Service} from '~c/api/api'
import RPCCaller from "~c/api/RPCCaller"
import SchemaView, {Dialect} from '~c/api/SchemaView'
import {ProcessReload} from '~lib/client/client'
import JSONRPCConn, {NotificationMsg} from '~lib/client/jsonrpc'

interface Props {
  appID: string;
  conn: JSONRPCConn;
}

interface State {
  meta?: APIMeta;
}

export default class AppAPI extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = {}
    this.onNotification = this.onNotification.bind(this)
  }

  componentDidMount() {
    this.props.conn.on("notification", this.onNotification)
    this.props.conn.request("status", {appID: this.props.appID}).then((status: any) => {
      if (status.meta) {
        this.setState({meta: status.meta})
      }
    })
  }

  componentWillUnmount() {
    this.props.conn.off("notification", this.onNotification)
  }

  onNotification(msg: NotificationMsg) {
    if (msg.method === "process/reload") {
      const data = msg.params as ProcessReload
      if (data.appID === this.props.appID) {
        this.setState({meta: data.meta})
      }
    }
  }

  render() {
    return (
      <div className="flex flex-col">
        {this.state.meta ? this.renderAPI() : (
          <div className="flex flex-col h-full items-center justify-center">
            <div className="text-gray-400 text-lg px-4 mb-6">
              No API schema available yet.
            </div>
          </div>
        )}
      </div>
    )
  }

  renderAPI() {
    const meta = this.state.meta!
    const svcPkg = (svc: Service) => {
      return meta.pkgs.find(pkg => pkg.rel_path === svc.rel_path)!
    }

    return (
      <div className="flex-grow flex items-start relative">
        <div className="w-56 flex-none hidden md:block sticky top-4">
          <SvcMenu svcs={meta.svcs} />
        </div>
        <div className="md:ml-6 flex-grow min-w-0 bg-white rounded-lg shadow-lg h-full">
          {meta.svcs.map(svc => {
            const rootPkg = svcPkg(svc)
            return (
              <div key={svc.name} className="border-b border-gray-300 px-4 md:px-8 lg:px-16">
                <div className="py-4 md:py-8 lg:py-16">
                  <h2 className="text-3xl leading-10 font-bold text-gray-600">Service {svc.name}</h2>
                  {rootPkg.doc &&
                    <p className="mt-4 mb-6 leading-6">{rootPkg.doc}</p>
                  }
                </div>

                {svc.rpcs.map(rpc => {
                  const pathParams = rpc.path.segments.filter(p => p.type !== "LITERAL")

                  let defaultMethod = rpc.http_methods[0]
                  if (defaultMethod === "*") {
                    defaultMethod = "POST"
                  } else if (defaultMethod !== "POST") {
                    for (const method of rpc.http_methods) {
                      if (method === "POST") {
                        defaultMethod = method
                        break
                      }
                    }
                  }

                  return (
                    <div key={rpc.name} className="border-t border-gray-300 py-10">
                      <h3 id={svc.name+"."+rpc.name} className="text-2xl leading-10 font-sans font-medium text-gray-700 flex items-center">
                        <span className="flex-grow truncate min-w-0">func {rpc.name}</span>
                      </h3>
                      <div className="md:flex md:items-stretch">
                        <div className="flex-initial min-w-0 md:mr-12 w-full" style={{maxWidth: "600px"}}>
                          {rpc.doc &&
                            <p className="mb-6 leading-6">{rpc.doc}</p>
                          }

                          {rpc.proto === "RAW" ? (
                            <div className="mt-4">
                              <div className="flex leading-6 text-gray-900 text-sm">
                                This API processes unstructured HTTP requests and therefore has no explicit schema.
                              </div>
                            </div>
                          ) : (
                            <>
                              {pathParams.length > 0 &&
                                <div className="mt-4">
                                  <h4 className="font-medium font-sans text-gray-700">Path Parameters</h4>
                                  <hr className="my-4 border-gray-200" />
                                  <div>
                                    {pathParams.map((p, i) =>
                                      <div key={p.value} className={i > 0 ? "border-t border-gray-200" : ""}>
                                        <div className="flex leading-6 font-mono">
                                          <div className="font-bold text-gray-900 text-sm">{p.value}</div>
                                          <div className="ml-2 text-xs text-gray-500">string</div>
                                        </div>
                                      </div>
                                    )}
                                  </div>
                                </div>
                              }

                              <div className="mt-4">
                                <h4 className="font-medium font-sans text-gray-700">Request</h4>
                                <hr className="my-4 border-gray-200" />
                                {rpc.request_schema ? <SchemaView meta={meta} service={svc} rpc={rpc} method={defaultMethod} type={rpc.request_schema} dialect="table" /> :
                                  <div className="text-gray-400 text-sm">No parameters.</div>
                                }
                              </div>

                              <div className="mt-12">
                                <h4 className="font-medium font-sans text-gray-700">Response</h4>
                                <hr className="my-4 border-gray-200" />
                                {rpc.response_schema ? <SchemaView meta={meta} service={svc} rpc={rpc} method={defaultMethod} type={rpc.response_schema} dialect="table" asResponse /> :
                                  <div className="text-gray-400 text-sm">No response.</div>
                                }
                              </div>
                            </>
                          )}
                        </div>
                        {rpc.proto !== "RAW" &&
                          <div className="flex-initial min-w-0 mt-10 md:mt-0 w-full" style={{maxWidth: "600px"}}>
                            <div className="sticky top-4">
                              <RPCDemo conn={this.props.conn} appID={this.props.appID} meta={meta} svc={svc} rpc={rpc} />
                            </div>
                          </div>
                        }
                      </div>
                    </div>
                  )
                })}
              </div>
            )
          })}
        </div>
      </div>
    )
  }
}

interface RPCDemoProps {
  conn: JSONRPCConn;
  appID: string;
  meta: APIMeta;
  svc: Service;
  rpc: RPC;
}

const RPCDemo: FunctionComponent<RPCDemoProps> = (props) => {
  const [respDialect, setRespDialect] = useState("json" as Dialect)

  type TabType = "schema" | "call"
  const [selectedTab, setSelectedTab] = useState<TabType>("schema")
  const tabs: {title: string; type: TabType}[] = [
    {title: "Schema", type: "schema"},
    {title: "Call", type: "call"},
  ]

  let defaultMethod = props.rpc.http_methods[0]
  if (defaultMethod === "*") {
    defaultMethod = "POST"
  } else if (defaultMethod !== "POST") {
    for (const method of props.rpc.http_methods) {
      if (method === "POST") {
        defaultMethod = method
        break
      }
    }
  }

    return <div>
      <nav className="flex justify-end space-x-4 mb-3">
        {tabs.map(t =>
          <button key={t.type} className={`px-3 py-2 font-medium text-sm rounded-md focus:outline-none ${selectedTab === t.type ? "text-purple-700 bg-purple-100" : "text-gray-500 hover:text-gray-700"}`}
              onClick={() => setSelectedTab(t.type)}>
            {t.title}
          </button>
        )}
      </nav>

      {selectedTab === "schema" ? <>
      {props.rpc.request_schema &&
        <div className="rounded-md border-gray-200 shadow request-docs">
          <div className="rounded-t-md bg-gray-600 p-2 uppercase text-gray-200 font-header text-xs tracking-wider flex">
            <div className="flex-grow">Request</div>
            <div className="flex-shrink-0">
              <select value={respDialect} onChange={(e) => setRespDialect(e.target.value as Dialect)}
                  className="form-select h-full py-0 border-transparent bg-transparent text-gray-300 text-xs leading-none">
                <option value="json">JSON</option>
                <option value="go">Go</option>
                <option value="curl">curl</option>
                {/*<option value="typescript">TypeScript</option>*/}
              </select>
            </div>
          </div>
          <div className="rounded-b-md bg-gray-800 text-gray-100 p-2 font-mono overflow-auto">
            <SchemaView meta={props.meta} service={props.svc} rpc={props.rpc} type={props.rpc.request_schema} method={defaultMethod} dialect={respDialect} />
          </div>
        </div>
      }
      {props.rpc.response_schema &&
        <div className="rounded-md border-gray-200 shadow mt-6 response-docs">
          <div className="rounded-t-md bg-gray-200 p-2 uppercase text-gray-600 font-header text-xs tracking-wider flex">
            <div className="flex-grow">Response</div>
            <div className="flex-shrink-0">
              <select value={respDialect} onChange={(e) => setRespDialect(e.target.value as Dialect)}
                  className="form-select h-full py-0 border-transparent bg-transparent text-gray-500 text-xs leading-none">
                <option value="json">JSON</option>
                <option value="go">Go</option>
                <option value="curl">curl</option>
                {/*<option value="typescript">TypeScript</option>*/}
              </select>
            </div>
          </div>
          <div className="rounded-b-md bg-gray-100 p-2 font-mono overflow-auto">
            <SchemaView meta={props.meta} service={props.svc} rpc={props.rpc} type={props.rpc.response_schema} method={defaultMethod} dialect={respDialect} asResponse />
          </div>
        </div>
      }
    </> : (
      <RPCCaller conn={props.conn} appID={props.appID} md={props.meta} svc={props.svc} rpc={props.rpc} />
    )}
  </div>
}

interface SvcMenuProps {
  svcs: Service[];
}

const SvcMenu: FunctionComponent<SvcMenuProps> = (props) => {
  return <>
    {props.svcs.map((svc, i) =>
      <div key={svc.name} className={(i > 0) ? "border-t border-gray-300" : ""}>
        <div className="flex w-full text-left px-4 py-2 text-gray-900">
          <div className="flex-grow flex">
            <div className="flex-grow text-sm text-gray-800 font-medium leading-5">{svc.name}</div>
            <div className="text-xs text-gray-400 flex-shrink-0 font-light">Service</div>
          </div>
        </div>
        <div className="py-1">
          {svc.rpcs.map((rpc, j) =>
            <a key={j} href={`#${svc.name}.${rpc.name}`} className="block pl-6 pr-4 py-2 text-sm leading-5 rounded-sm text-gray-700 hover:bg-gray-300 hover:text-gray-900 focus:outline-none focus:bg-gray-100 focus:text-gray-900">
              {rpc.name}
            </a>
          )}
        </div>
      </div>
    )}
  </>
}
