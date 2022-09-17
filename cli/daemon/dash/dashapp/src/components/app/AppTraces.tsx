import React, { FC, useState } from "react";
import * as icons from "~c/icons";
import { Modal } from "~c/Modal";
import { Request, Stack, Trace } from "~c/trace/model";
import SpanDetail from "~c/trace/SpanDetail";
import SpanList from "~c/trace/SpanList";
import StackTrace from "~c/trace/StackTrace";
import TraceMap from "~c/trace/TraceMap";
import { latencyStr } from "~c/trace/util";
import { decodeBase64 } from "~lib/base64";
import JSONRPCConn, { NotificationMsg } from "~lib/client/jsonrpc";
import { timeToDate } from "~lib/time";
import { Icon } from "~c/icons";

interface Props {
  appID: string;
  conn: JSONRPCConn;
}

interface State {
  traces: Trace[];
  selected?: Trace;
}

export default class AppTraces extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { traces: [] };
    this.onNotification = this.onNotification.bind(this);
  }

  componentDidMount() {
    this.props.conn.on("notification", this.onNotification);
    this.props.conn
      .request("list-traces", { appID: this.props.appID })
      .then((traces) => {
        this.setState({ traces: (traces as Trace[]).reverse() });
      });
  }

  componentWillUnmount() {
    this.props.conn.off("notification", this.onNotification);
  }

  onNotification(msg: NotificationMsg) {
    if (msg.method === "trace/new") {
      const tr = msg.params as Trace;
      this.setState((st) => {
        return { traces: [tr, ...st.traces] };
      });
    }
  }

  render() {
    return (
      <div className="flex flex-col">
        <Modal
          show={this.state.selected !== undefined}
          close={() => this.setState({ selected: undefined })}
          width="w-full h-full mt-4"
        >
          {this.state.selected && (
            <TraceView
              trace={this.state.selected}
              close={() => this.setState({ selected: undefined })}
            />
          )}
        </Modal>

        <div className="overflow-hidden bg-white shadow sm:rounded-md">
          {this.state.traces.length === 0 && (
            <div className="p-4">
              No traces yet. Make an API call to see it here!
            </div>
          )}
          <ul className="divide-y divide-gray-200">
            {this.state.traces.map((tr) => {
              const loc = tr.locations[(tr.root ?? tr.auth)!.def_loc];
              let icon: Icon = icons.exclamation;
              let endpoint = "<unknown endpoint>";
              let type = "<unknown request type>";

              if ("rpc_def" in loc) {
                endpoint =
                  loc.rpc_def.service_name + "." + loc.rpc_def.rpc_name;
                icon = icons.logout;
                type = "API Call";
              } else if ("auth_handler_def" in loc) {
                endpoint =
                  loc.auth_handler_def.service_name +
                  "." +
                  loc.auth_handler_def.name;
                icon = icons.shield;
                type = "Auth Call";
              } else if ("pubsub_subscriber" in loc) {
                endpoint =
                  loc.pubsub_subscriber.topic_name +
                  "." +
                  loc.pubsub_subscriber.subscriber_name;
                icon = icons.arrowsExpand;
                type = "PubSub Message Received";
              }

              return (
                <li key={tr.id}>
                  <div
                    className="px-4 py-4 hover:bg-gray-50 sm:px-6"
                    onClick={() => this.setState({ selected: tr })}
                  >
                    <div className="flex items-center justify-between">
                      <p className="truncate text-base font-medium text-gray-800">
                        {icon("h-4 w-4 inline-block mr-2", type)}
                        {endpoint}
                      </p>
                      <div className="ml-2 flex flex-shrink-0">
                        {tr.root?.err === null ? (
                          <span className="inline-flex rounded-full bg-green-100 px-2 text-xs font-semibold leading-5 text-green-800">
                            Success
                          </span>
                        ) : (
                          <span className="inline-flex rounded-full bg-red-100 px-2 text-xs font-semibold leading-5 text-red-800">
                            Error
                          </span>
                        )}
                      </div>
                    </div>
                    <div className="mt-2 sm:flex sm:justify-between">
                      <div className="sm:flex">
                        <p className="flex cursor-pointer items-center truncate text-sm font-medium text-indigo-600 hover:underline">
                          <svg
                            className="mr-1 h-4 w-4"
                            fill="none"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth="2"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <polyline points="2 14.308 5.076 14.308 8.154 2 11.231 20.462 14.308 9.692 15.846 14.308 18.924 14.308" />
                            <circle cx="20.462" cy="14.308" r="1.538" />
                          </svg>
                          View Trace
                        </p>
                      </div>
                      <div className="mt-2 flex items-center text-sm text-gray-500 sm:mt-0">
                        <svg
                          className="mr-1 h-4 w-4"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                          />
                        </svg>
                        {tr.end_time
                          ? latencyStr(tr.end_time - tr.start_time)
                          : "Unknown"}
                      </div>
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      </div>
    );
  }
}

interface TraceViewProps {
  trace: Trace;
  close: () => void;
}

const TraceView: FC<TraceViewProps> = (props) => {
  const tr = props.trace;
  const dt = timeToDate(tr.date)!;
  const [selected, setSelected] = useState<Request>((tr.root ?? tr.auth)!);
  const [stack, setStack] = useState<Stack | undefined>(undefined);

  return (
    <section className="relative flex h-full flex-grow items-stretch bg-white">
      <div className="absolute -top-2 -right-2">
        <div
          className="cursor-pointer rounded-full p-1 hover:bg-gray-100"
          onClick={() => props.close()}
        >
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </div>
      </div>

      <div className="flex flex-grow flex-col overflow-scroll">
        <div className="flex border-b border-gray-100 p-4">
          <div className="mr-4 flex-shrink-0">
            <h1 className="mb-1 text-2xl font-bold leading-none text-gray-900">
              Trace Details
            </h1>
            <table className="text-sm">
              <tbody>
                <tr>
                  <th className="pr-2 text-left text-sm font-light text-gray-400">
                    Recorded
                  </th>
                  <td>{dt.toFormat("ff")}</td>
                </tr>
                {tr.auth !== null && tr.auth.err === null && (
                  <>
                    <tr className="text-left font-normal">
                      <th className="pr-2 text-left text-sm font-light text-gray-400">
                        User ID
                      </th>
                      <td className="font-mono">
                        {JSON.parse(decodeBase64(tr.auth.outputs[0]))}
                      </td>
                    </tr>
                  </>
                )}
              </tbody>
            </table>
          </div>
          <div className="flex-grow">
            <TraceMap trace={tr} selected={selected} onSelect={setSelected} />
          </div>
        </div>

        <div className="relative mt-4 flex flex-col">
          {stack ? (
            <div className="mr-4">
              <h3 className="mb-2 flex items-center justify-between text-xl font-semibold">
                Stack Trace
                <button
                  className="focus:outline-none hover:text-gray-600"
                  onClick={() => setStack(undefined)}
                >
                  {icons.x("h-5 w-5")}
                </button>
              </h3>
              <StackTrace stack={stack} />
            </div>
          ) : (
            <>
              <h3 className="mb-2 text-xl font-semibold">Request Tree</h3>
              <SpanList trace={tr} selected={selected} onSelect={setSelected} />
            </>
          )}
        </div>
      </div>

      <div className="w-96 flex-shrink-0 overflow-scroll border-l border-gray-100 p-4 md:w-1/2">
        <SpanDetail req={selected} trace={tr} onStackTrace={setStack} />
      </div>
    </section>
  );
};
