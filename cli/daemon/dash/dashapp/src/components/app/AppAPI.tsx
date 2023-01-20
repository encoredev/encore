import React, { FunctionComponent, useState } from "react";
import { APIEncoding, APIMeta, RPC, Service } from "~c/api/api";
import RPCCaller from "~c/api/RPCCaller";
import SchemaView, { Dialect } from "~c/api/SchemaView";
import { ProcessReload } from "~lib/client/client";
import JSONRPCConn, { NotificationMsg } from "~lib/client/jsonrpc";
import APINav from "~c/api/APINav";

interface Props {
  appID: string;
  conn: JSONRPCConn;
}

interface State {
  meta?: APIMeta;
  apiEncoding?: APIEncoding;
}

export default class AppAPI extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {};
    this.onNotification = this.onNotification.bind(this);
  }

  componentDidMount() {
    this.props.conn.on("notification", this.onNotification);
    this.props.conn.request("status", { appID: this.props.appID }).then((status: any) => {
      if (status.meta && status.apiEncoding) {
        this.setState({ meta: status.meta, apiEncoding: status.apiEncoding });
      }
    });
  }

  componentWillUnmount() {
    this.props.conn.off("notification", this.onNotification);
  }

  onNotification(msg: NotificationMsg) {
    if (msg.method === "process/reload") {
      const data = msg.params as ProcessReload;
      if (data.appID === this.props.appID) {
        this.setState({
          meta: data.meta,
          apiEncoding: data.apiEncoding,
        });
      }
    }
  }

  render() {
    return this.state.meta && this.state.apiEncoding ? (
      this.renderAPI()
    ) : (
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex h-full min-h-0 flex-1 flex-col items-center justify-center">
          <div className="text-gray-400 mb-6 px-4 text-lg">
            No API schema available yet. Create an endpoint to view it here
          </div>
        </div>
      </div>
    );
  }

  renderAPI() {
    const meta = this.state.meta!;
    const apiEncoding = this.state.apiEncoding!;
    const svcPkg = (svc: Service) => {
      return meta.pkgs.find((pkg) => pkg.rel_path === svc.rel_path)!;
    };

    return (
      <div className="flex min-h-0 w-full flex-1 flex-col overflow-auto">
        <div className="flex min-w-0 flex-1 items-stretch overflow-auto">
          <div
            className={`hidden h-full-minus-nav w-64 flex-shrink-0 overflow-auto bg-black text-white lg:flex lg:flex-col`}
          >
            <APINav meta={meta} />
          </div>
          <div className={`flex h-full-minus-nav min-w-0 flex-1 flex-col overflow-auto`}>
            <div className="min-h-0 flex-grow rounded-lg p-4">
              {meta.svcs.map((svc, i) => {
                const rootPkg = svcPkg(svc);
                return (
                  <div key={i} className="px-4">
                    <div className="py-4">
                      <h2 className="text-gray-600 font-sans text-3xl leading-10">
                        Service {svc.name}
                      </h2>
                      {rootPkg.doc && <p className="mt-4 mb-6 leading-6">{rootPkg.doc}</p>}
                    </div>

                    {svc.rpcs.map((rpc) => {
                      const pathParams = rpc.path.segments.filter((p) => p.type !== "LITERAL");

                      let defaultMethod = rpc.http_methods[0];
                      if (defaultMethod === "*") {
                        defaultMethod = "POST";
                      } else if (defaultMethod !== "POST") {
                        for (const method of rpc.http_methods) {
                          if (method === "POST") {
                            defaultMethod = method;
                            break;
                          }
                        }
                      }

                      return (
                        <div key={i} className="py-4">
                          <div className="md:flex md:items-stretch">
                            <div
                              className="w-full min-w-0 flex-initial md:mr-12"
                              style={{ maxWidth: "600px" }}
                            >
                              <h3
                                id={svc.name + "." + rpc.name}
                                className="text-gray-700 flex min-w-0 items-center font-sans text-2xl leading-10"
                              >
                                <span className="flex-shrink truncate">func {rpc.name} </span>
                                {rpc.access_type === "PUBLIC" ? (
                                  <span
                                    className="lead-xs ml-4 inline-flex flex-none items-center rounded bg-codegreen px-2 py-1.5 leading-none text-black"
                                    title="This API is publicly accessible to anyone on the internet"
                                  >
                                    Public
                                  </span>
                                ) : rpc.access_type === "AUTH" ? (
                                  <span
                                    className="lead-xs ml-4 inline-flex flex-none items-center rounded bg-codeyellow px-2 py-1.5 leading-none text-black"
                                    title="This API requires authentication by the client"
                                  >
                                    Auth
                                  </span>
                                ) : (
                                  <span
                                    className="lead-xs ml-4 inline-flex flex-none items-center rounded bg-codeorange px-2 py-1.5 leading-none text-black"
                                    title="This API is only accessible by your organization"
                                  >
                                    Private
                                  </span>
                                )}
                              </h3>
                              {rpc.doc && <p className="mb-6 text-xs">{rpc.doc}</p>}

                              {rpc.proto === "RAW" ? (
                                <div className="mt-4">
                                  <div className="text-gray-900 flex text-sm leading-6">
                                    This API processes unstructured HTTP requests and therefore has
                                    no explicit schema.
                                  </div>
                                </div>
                              ) : (
                                <>
                                  {pathParams.length > 0 && (
                                    <div className="mt-4">
                                      <h4 className="mb-2 border-b border-black border-opacity-[15%] pb-2 font-sans text-black">
                                        Path Parameters
                                      </h4>
                                      <div>
                                        {pathParams.map((p, i) => (
                                          <div key={p.value}>
                                            <div className="flex items-center font-mono leading-6">
                                              <div className="text-gray-900 text-sm font-bold">
                                                {p.value}
                                              </div>
                                              <div className="text-gray-500 ml-2 text-xs">
                                                string
                                              </div>
                                            </div>
                                          </div>
                                        ))}
                                      </div>
                                      {rpc.request_schema ? (
                                        <SchemaView
                                          meta={meta}
                                          service={svc}
                                          rpc={rpc}
                                          type={rpc.request_schema}
                                          method={defaultMethod}
                                          dialect="table"
                                        />
                                      ) : (
                                        <div className="text-gray-400 text-sm">No parameters.</div>
                                      )}
                                    </div>
                                  )}

                                  <div className="mt-4">
                                    <h4 className="mb-2 border-b border-black border-opacity-[15%] pb-2 font-sans text-black">
                                      Request
                                    </h4>
                                    {rpc.request_schema ? (
                                      <SchemaView
                                        meta={meta}
                                        service={svc}
                                        rpc={rpc}
                                        type={rpc.request_schema}
                                        method={defaultMethod}
                                        dialect="table"
                                      />
                                    ) : (
                                      <div className="text-gray-400 text-sm">No parameters.</div>
                                    )}
                                  </div>

                                  <div className="mt-12">
                                    <h4 className="mb-2 border-b border-black border-opacity-[15%] pb-2 font-sans text-black">
                                      Response
                                    </h4>
                                    {rpc.response_schema ? (
                                      <SchemaView
                                        meta={meta}
                                        service={svc}
                                        rpc={rpc}
                                        type={rpc.response_schema}
                                        method={defaultMethod}
                                        dialect="table"
                                        asResponse
                                      />
                                    ) : (
                                      <div className="text-gray-400 text-sm">No response.</div>
                                    )}
                                  </div>
                                </>
                              )}
                            </div>
                            {rpc.proto !== "RAW" && (
                              <div
                                className="mt-10 w-full min-w-0 flex-initial md:mt-0"
                                style={{ maxWidth: "600px" }}
                              >
                                <div className="sticky top-4">
                                  <RPCDemo
                                    conn={this.props.conn}
                                    appID={this.props.appID}
                                    meta={meta}
                                    apiEncoding={apiEncoding}
                                    svc={svc}
                                    rpc={rpc}
                                  />
                                </div>
                              </div>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    );
  }
}

interface RPCDemoProps {
  conn: JSONRPCConn;
  appID: string;
  meta: APIMeta;
  apiEncoding: APIEncoding;
  svc: Service;
  rpc: RPC;
}

const RPCDemo: FunctionComponent<RPCDemoProps> = (props) => {
  const [respDialect, setRespDialect] = useState("json" as Dialect);

  type TabType = "schema" | "call";
  const [selectedTab, setSelectedTab] = useState<TabType>("schema");
  const tabs: { title: string; type: TabType }[] = [
    { title: "Schema", type: "schema" },
    { title: "Call", type: "call" },
  ];

  let defaultMethod = props.rpc.http_methods[0];
  if (defaultMethod === "*") {
    defaultMethod = "POST";
  } else if (defaultMethod !== "POST") {
    for (const method of props.rpc.http_methods) {
      if (method === "POST") {
        defaultMethod = method;
        break;
      }
    }
  }

  return (
    <div>
      <nav className="mb-3 flex justify-end space-x-4">
        {tabs.map((t) => (
          <button
            key={t.type}
            className={`lead-xs lead-small rounded px-5 py-2 uppercase focus:outline-none ${
              selectedTab === t.type
                ? "bg-black text-white"
                : "border border-black bg-white text-black"
            }`}
            onClick={() => setSelectedTab(t.type)}
          >
            {t.title}
          </button>
        ))}
      </nav>

      {selectedTab === "schema" ? (
        <>
          {props.rpc.request_schema && (
            <div className="request-docs rounded-md border border-black">
              <div className="flex rounded-t bg-white p-2 text-xs tracking-wider text-black">
                <div className="lead-xs flex-grow uppercase">Request</div>
                <div className="flex-shrink-0">
                  <select
                    value={respDialect}
                    onChange={(e) => setRespDialect(e.target.value as Dialect)}
                    className="form-select h-full !border-transparent bg-transparent py-0 text-right font-mono text-xs leading-none text-black focus:outline-none focus:ring-0"
                  >
                    <option value="json">JSON</option>
                    <option value="go">Go</option>
                    <option value="curl">curl</option>
                    {/*<option value="typescript">TypeScript</option>*/}
                  </select>
                </div>
              </div>
              <div className="code-xs overflow-auto rounded-b bg-black p-2 text-white">
                <SchemaView
                  meta={props.meta}
                  service={props.svc}
                  rpc={props.rpc}
                  type={props.rpc.request_schema}
                  method={defaultMethod}
                  dialect={respDialect}
                />
              </div>
            </div>
          )}
          {props.rpc.response_schema && (
            <div className="response-docs mt-6 rounded-md border border-black">
              <div className="flex rounded-t bg-white p-2 text-xs uppercase tracking-wider text-black">
                <div className="lead-xs flex-grow uppercase">Response</div>
                <div className="flex-shrink-0">
                  <select
                    value={respDialect}
                    onChange={(e) => setRespDialect(e.target.value as Dialect)}
                    className="form-select h-full !border-transparent bg-transparent py-0 text-right font-mono text-xs leading-none text-black focus:outline-none focus:ring-0"
                  >
                    <option value="json">JSON</option>
                    <option value="go">Go</option>
                    <option value="curl">curl</option>
                    {/*<option value="typescript">TypeScript</option>*/}
                  </select>
                </div>
              </div>
              <div className="code-xs overflow-auto rounded-b bg-black p-2 text-white">
                <SchemaView
                  meta={props.meta}
                  service={props.svc}
                  rpc={props.rpc}
                  type={props.rpc.response_schema}
                  method={defaultMethod}
                  dialect={respDialect}
                  asResponse
                />
              </div>
            </div>
          )}
        </>
      ) : (
        <RPCCaller
          conn={props.conn}
          appID={props.appID}
          md={props.meta}
          apiEncoding={props.apiEncoding}
          svc={props.svc}
          rpc={props.rpc}
        />
      )}
    </div>
  );
};
