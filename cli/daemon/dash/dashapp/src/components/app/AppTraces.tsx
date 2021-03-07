import React from 'react'
import { Modal } from '~c/Modal'
import { Request, Trace } from '~c/trace/model'
import SpanDetail from '~c/trace/SpanDetail'
import SpanList from '~c/trace/SpanList'
import TraceMap from '~c/trace/TraceMap'
import { latencyStr } from '~c/trace/util'
import { decodeBase64 } from '~lib/base64'
import JSONRPCConn, { NotificationMsg } from '~lib/client/jsonrpc'
import { timeToDate } from '~lib/time'
import parseAnsi, { Chunk } from "~lib/parse-ansi"

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
    super(props)
    this.state = {traces: []}
    this.onNotification = this.onNotification.bind(this)
  }

  async componentDidMount() {
    const traces = await this.props.conn.request("list-traces", {appID: this.props.appID}) as Trace[]
    this.setState({traces: traces.reverse()})
    this.props.conn.on("notification", this.onNotification)
  }

  componentWillUnmount() {
    this.props.conn.off("notification", this.onNotification)
  }

  onNotification(msg: NotificationMsg) {
    if (msg.method === "trace/new") {
      const tr = msg.params as Trace
      this.setState((st) => {
        return {traces: [tr, ...st.traces]}
      })
    }
  }

  render() {
    return (
      <div className="flex flex-col mt-2">
        <Modal show={this.state.selected !== undefined} close={() => this.setState({selected: undefined})} width="w-full h-full mt-4">
          {this.state.selected && <TraceView trace={this.state.selected} close={() => this.setState({selected: undefined})} /> }
        </Modal>

        <div className="align-middle min-w-full overflow-x-auto shadow overflow-hidden sm:rounded-lg">
          <table className="min-w-full divide-y divide-cool-gray-200">
            <thead>
              <tr>
                <th className="px-6 py-3 bg-cool-gray-50 text-left text-xs leading-4 font-medium text-cool-gray-500 uppercase tracking-wider">
                  Request
                </th>
                <th className="px-6 py-3 bg-cool-gray-50 text-left text-xs leading-4 font-medium text-cool-gray-500 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 bg-cool-gray-50 text-right text-xs leading-4 font-medium text-cool-gray-500 uppercase tracking-wider">
                  Duration
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-cool-gray-200">
              {this.state.traces.map(tr => {
                const loc = tr.locations[tr.root.def_loc]
                let endpoint = "<unknown endpoint>"
                if ("rpc_def" in loc) {
                  endpoint = loc.rpc_def.service_name + "." + loc.rpc_def.rpc_name
                }

                return (
                  <tr key={tr.id} className="bg-white">
                    <td className="max-w-0 w-full px-6 py-4 whitespace-no-wrap text-sm leading-5 text-cool-gray-900">
                      <div className="flex">
                        <div className="group inline-flex space-x-2 truncate text-sm leading-5 cursor-pointer" onClick={() => this.setState({selected: tr})}>
                          <p className="text-cool-gray-500 truncate group-hover:text-cool-gray-900 transition ease-in-out duration-150">
                            {endpoint}
                          </p>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-no-wrap text-sm leading-5 text-cool-gray-500">
                      {tr.root.err === null ? (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium leading-4 bg-green-100 text-green-800 capitalize">
                          success
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium leading-4 bg-red-100 text-red-800 capitalize">
                          error
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4 text-right whitespace-no-wrap text-sm leading-5 text-cool-gray-500">
                      {latencyStr(tr.end_time - tr.start_time)}
                    </td>
                  </tr>
                )
              })}

            </tbody>
          </table>
          
        </div>
      </div>
    )
  }
}

interface TraceViewProps {
  trace: Trace;
  close: () => void;
}

interface TraceViewState {
  selected: Request;
}

class TraceView extends React.Component<TraceViewProps, TraceViewState> {
  constructor(props: TraceViewProps) {
    super(props)
    this.state = {
      selected: props.trace.root
    }
  }

  render() {
    const tr = this.props.trace
    const dt = timeToDate(tr.date)!

    return (
      <section className="bg-white flex-grow flex items-stretch h-full">

        <div className="flex-grow flex flex-col overflow-scroll">
          <div className="flex p-4 border-b border-gray-100">
            <div className="flex-shrink-0 mr-4">
              <h1 className="text-2xl font-bold leading-none text-gray-900 mb-1">
                Trace Details
              </h1>
              <table className="text-sm">
                <tbody>
                  <tr>
                    <th className="text-left text-sm font-light text-gray-400 pr-2">Recorded</th>
                    <td>{dt.toFormat("ff")}</td>
                  </tr>
                  {tr.auth !== null && <>
                    <tr className="text-left font-normal">
                      <th className="text-left text-sm font-light text-gray-400 pr-2">User ID</th>
                      <td className="font-mono">{JSON.parse(decodeBase64(tr.auth.outputs[0]))}</td>
                    </tr>
                  </>}
                </tbody>
              </table>
            </div>
            <div className="flex-grow">
              <TraceMap trace={this.props.trace!} selected={this.state.selected}
                  onSelect={(req: Request) => this.setState({selected: req})} />
            </div>
          </div>

          <div className="flex flex-col mt-4">
            <SpanList trace={tr} selected={this.state.selected}
                onSelect={(req: Request) => this.setState({selected: req})} />
          </div>
        </div>

        <div className="flex-shrink-0 w-96 md:w-1/2 p-4 border-l border-gray-100 overflow-scroll">
          <SpanDetail req={this.state.selected!} trace={tr} />
        </div>
      </section>
    )
  }
}