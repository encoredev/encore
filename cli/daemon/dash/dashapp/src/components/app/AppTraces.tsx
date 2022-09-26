import React, { FC, useState } from "react";
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
import { Icon, icons } from "~c/icons";

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
          width="w-full h-full mt-6"
        >
          {this.state.selected && (
            <TraceView
              trace={this.state.selected}
              close={() => this.setState({ selected: undefined })}
            />
          )}
        </Modal>

        <div className="shadow overflow-hidden bg-white sm:rounded-md">
          <ul>
            <li className="flex items-center py-4 pt-2 text-left text-xs font-medium uppercase leading-4 tracking-wider">
              <p className="flex flex-1 items-center">Request</p>
              <p className="flex min-w-[80px]">Status</p>
              <p className="flex min-w-[80px] items-center justify-end">
                Duration
              </p>
            </li>

            {this.state.traces.length === 0 && (
              <div>No traces yet. Make an API call to see it here!</div>
            )}

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
                <li key={tr.id} className="py-4">
                  <div className="hover:bg-gray-50">
                    <div className="flex items-center">
                      <p className="text-gray-800 flex-1 truncate text-base font-medium">
                        {icon("h-4 w-4 inline-block mr-2", type)}
                        <a
                          className="cursor-pointer brandient-5 link-brandient"
                          onClick={() => this.setState({ selected: tr })}
                        >
                          <span>{endpoint}</span>
                        </a>
                      </p>
                      <div className="ml-2 flex w-[80px]">
                        {tr.root?.err === null ? (
                          <span className="inline-flex items-center rounded bg-codegreen px-2.5 py-0.5 text-xs font-medium capitalize leading-4">
                            Success
                          </span>
                        ) : (
                          <span className="inline-flex items-center rounded bg-validation-fail px-2.5 py-0.5 text-xs font-medium capitalize leading-4 text-white">
                            Error
                          </span>
                        )}
                      </div>
                      <div className="text-gray-500 mt-2 flex min-w-[80px] items-center justify-end text-sm sm:mt-0">
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
      <div className="absolute -top-2 -right-2 bg-white">
        <div
          className="hover:bg-gray-100 cursor-pointer rounded-full p-1"
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

      <div className="flex flex-grow flex-col overflow-auto">
        <div className="border-gray-100 flex border-b p-4">
          <div className="mr-4 flex-shrink-0">
            <h1 className="text-gray-900 mb-1 text-2xl font-semibold leading-none">
              Trace Details
            </h1>
            <table className="text-sm">
              <tbody>
                <tr>
                  <th className="pr-2 text-left text-sm font-semibold text-black">
                    Recorded
                  </th>
                  <td>{dt.toFormat("ff")}</td>
                </tr>
                {tr.auth !== null && tr.auth.err === null && (
                  <>
                    <tr className="text-left font-normal">
                      <th className="text-gray-400 pr-2 text-left text-sm font-light">
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
                  className="hover:text-gray-600 focus:outline-none"
                  onClick={() => setStack(undefined)}
                >
                  {icons.x("h-5 w-5")}
                </button>
              </h3>
              <StackTrace stack={stack} />
            </div>
          ) : (
            <SpanList trace={tr} selected={selected} onSelect={setSelected} />
          )}
        </div>
      </div>

      <div className="h-full w-96 flex-shrink-0 border-l border-black p-4 md:w-1/2">
        <SpanDetail req={selected} trace={tr} onStackTrace={setStack} />
      </div>
    </section>
  );
};
