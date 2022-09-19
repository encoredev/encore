import React, { FunctionComponent, useState } from "react";
import { APIMeta, RPC, Service } from "~c/api/api";
import RPCCaller from "~c/api/RPCCaller";
import SchemaView, { Dialect } from "~c/api/SchemaView";
import { ProcessReload } from "~lib/client/client";
import JSONRPCConn, { NotificationMsg } from "~lib/client/jsonrpc";

interface Props {
  appID: string;
  conn: JSONRPCConn;
}

interface State {
  meta?: APIMeta;
}

export default class AppAPI extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {};
    this.onNotification = this.onNotification.bind(this);
  }

  componentDidMount() {
    this.props.conn.on("notification", this.onNotification);
    this.props.conn
      .request("status", { appID: this.props.appID })
      .then((status: any) => {
        if (status.meta) {
          this.setState({ meta: status.meta });
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
        this.setState({ meta: data.meta });
      }
    }
  }

  render() {
    return (
      <div className="flex flex-col">
        {this.state.meta ? (
          this.renderAPI()
        ) : (
          <div className="flex h-full flex-col items-center justify-center">
            <div className="text-gray-400 mb-6 px-4 text-lg">
              No API schema available yet.
            </div>
          </div>
        )}
      </div>
    );
  }

  renderAPI() {
    const meta = this.state.meta!;
    const svcPkg = (svc: Service) => {
      return meta.pkgs.find((pkg) => pkg.rel_path === svc.rel_path)!;
    };

    return (
      <div className="relative flex flex-grow items-start">
        <div className="sticky top-4 hidden w-56 flex-none md:block">
          <SvcMenu svcs={meta.svcs} />
        </div>
        <div className="shadow-lg h-full min-w-0 flex-grow rounded-lg bg-white md:ml-6">
          {meta.svcs.map((svc) => {
            const rootPkg = svcPkg(svc);
            return (
              <div
                key={svc.name}
                className="border-gray-300 border-b px-4 md:px-8 lg:px-16"
              >
                <div className="py-4 md:py-8 lg:py-16">
                  <h2 className="text-gray-600 text-3xl font-bold leading-10">
                    Service {svc.name}
                  </h2>
                  {rootPkg.doc && (
                    <p className="mt-4 mb-6 leading-6">{rootPkg.doc}</p>
                  )}
                </div>

                {svc.rpcs.map((rpc) => {
                  const pathParams = rpc.path.segments.filter(
                    (p) => p.type !== "LITERAL"
                  );

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
                    <div
                      key={rpc.name}
                      className="border-gray-300 border-t py-10"
                    >
                      <h3
                        id={svc.name + "." + rpc.name}
                        className="text-gray-700 flex items-center font-sans text-2xl font-medium leading-10"
                      >
                        <span className="min-w-0 flex-grow truncate">
                          func {rpc.name}
                        </span>
                      </h3>
                      <div className="md:flex md:items-stretch">
                        <div
                          className="w-full min-w-0 flex-initial md:mr-12"
                          style={{ maxWidth: "600px" }}
                        >
                          {rpc.doc && (
                            <p className="mb-6 leading-6">{rpc.doc}</p>
                          )}

                          {rpc.proto === "RAW" ? (
                            <div className="mt-4">
                              <div className="text-gray-900 flex text-sm leading-6">
                                This API processes unstructured HTTP requests
                                and therefore has no explicit schema.
                              </div>
                            </div>
                          ) : (
                            <>
                              {pathParams.length > 0 && (
                                <div className="mt-4">
                                  <h4 className="text-gray-700 font-sans font-medium">
                                    Path Parameters
                                  </h4>
                                  <hr className="border-gray-200 my-4" />
                                  <div>
                                    {pathParams.map((p, i) => (
                                      <div
                                        key={p.value}
                                        className={
                                          i > 0
                                            ? "border-gray-200 border-t"
                                            : ""
                                        }
                                      >
                                        <div className="flex font-mono leading-6">
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
                                </div>
                              )}

                              <div className="mt-4">
                                <h4 className="text-gray-700 font-sans font-medium">
                                  Request
                                </h4>
                                <hr className="border-gray-200 my-4" />
                                {rpc.request_schema ? (
                                  <SchemaView
                                    meta={meta}
                                    service={svc}
                                    rpc={rpc}
                                    method={defaultMethod}
                                    type={rpc.request_schema}
                                    dialect="table"
                                  />
                                ) : (
                                  <div className="text-gray-400 text-sm">
                                    No parameters.
                                  </div>
                                )}
                              </div>

                              <div className="mt-12">
                                <h4 className="text-gray-700 font-sans font-medium">
                                  Response
                                </h4>
                                <hr className="border-gray-200 my-4" />
                                {rpc.response_schema ? (
                                  <SchemaView
                                    meta={meta}
                                    service={svc}
                                    rpc={rpc}
                                    method={defaultMethod}
                                    type={rpc.response_schema}
                                    dialect="table"
                                    asResponse
                                  />
                                ) : (
                                  <div className="text-gray-400 text-sm">
                                    No response.
                                  </div>
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
    );
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
            className={`focus:outline-none rounded-md px-3 py-2 text-sm font-medium ${
              selectedTab === t.type
                ? "text-purple-700 bg-purple-100"
                : "text-gray-500 hover:text-gray-700"
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
            <div className="border-gray-200 shadow request-docs rounded-md">
              <div className="bg-gray-600 text-gray-200 flex rounded-t-md p-2 font-header text-xs uppercase tracking-wider">
                <div className="flex-grow">Request</div>
                <div className="flex-shrink-0">
                  <select
                    value={respDialect}
                    onChange={(e) => setRespDialect(e.target.value as Dialect)}
                    className="form-select text-gray-300 h-full border-transparent bg-transparent py-0 text-xs leading-none"
                  >
                    <option value="json">JSON</option>
                    <option value="go">Go</option>
                    <option value="curl">curl</option>
                    {/*<option value="typescript">TypeScript</option>*/}
                  </select>
                </div>
              </div>
              <div className="bg-gray-800 text-gray-100 overflow-auto rounded-b-md p-2 font-mono">
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
            <div className="border-gray-200 shadow response-docs mt-6 rounded-md">
              <div className="bg-gray-200 text-gray-600 flex rounded-t-md p-2 font-header text-xs uppercase tracking-wider">
                <div className="flex-grow">Response</div>
                <div className="flex-shrink-0">
                  <select
                    value={respDialect}
                    onChange={(e) => setRespDialect(e.target.value as Dialect)}
                    className="form-select text-gray-500 h-full border-transparent bg-transparent py-0 text-xs leading-none"
                  >
                    <option value="json">JSON</option>
                    <option value="go">Go</option>
                    <option value="curl">curl</option>
                    {/*<option value="typescript">TypeScript</option>*/}
                  </select>
                </div>
              </div>
              <div className="bg-gray-100 overflow-auto rounded-b-md p-2 font-mono">
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
          svc={props.svc}
          rpc={props.rpc}
        />
      )}
    </div>
  );
};

interface SvcMenuProps {
  svcs: Service[];
}

const SvcMenu: FunctionComponent<SvcMenuProps> = (props) => {
  return (
    <>
      {props.svcs.map((svc, i) => (
        <div key={svc.name} className={i > 0 ? "border-gray-300 border-t" : ""}>
          <div className="text-gray-900 flex w-full px-4 py-2 text-left">
            <div className="flex flex-grow">
              <div className="text-gray-800 flex-grow text-sm font-medium leading-5">
                {svc.name}
              </div>
              <div className="text-gray-400 flex-shrink-0 text-xs font-light">
                Service
              </div>
            </div>
          </div>
          <div className="py-1">
            {svc.rpcs.map((rpc, j) => (
              <a
                key={j}
                href={`#${svc.name}.${rpc.name}`}
                className="text-gray-700 hover:bg-gray-300 hover:text-gray-900 focus:outline-none focus:bg-gray-100 focus:text-gray-900 block rounded-sm py-2 pl-6 pr-4 text-sm leading-5"
              >
                {rpc.name}
              </a>
            ))}
          </div>
        </div>
      ))}
    </>
  );
};
