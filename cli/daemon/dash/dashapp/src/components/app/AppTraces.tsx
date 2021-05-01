import React, { FC, useState } from 'react'
import { Modal } from '~c/Modal'
import { Request, Stack, Trace } from '~c/trace/model'
import SpanDetail from '~c/trace/SpanDetail'
import SpanList from '~c/trace/SpanList'
import TraceMap from '~c/trace/TraceMap'
import StackTrace from '~c/trace/StackTrace'
import { latencyStr } from '~c/trace/util'
import { decodeBase64 } from '~lib/base64'
import JSONRPCConn, { NotificationMsg } from '~lib/client/jsonrpc'
import { timeToDate } from '~lib/time'
import * as icons from "~c/icons"

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
      <div className="flex flex-col">
        <Modal show={this.state.selected !== undefined} close={() => this.setState({selected: undefined})} width="w-full h-full mt-4">
          {this.state.selected && <TraceView trace={this.state.selected} close={() => this.setState({selected: undefined})} /> }
        </Modal>

        <div className="bg-white shadow overflow-hidden sm:rounded-md">
          {this.state.traces.length === 0 && (
            <div className="p-4">
              No traces yet. Make an API call to see it here!
            </div>
          )}
          <ul className="divide-y divide-gray-200">
            {this.state.traces.map(tr => {
              const loc = tr.locations[tr.root.def_loc]
              let endpoint = "<unknown endpoint>"
              if ("rpc_def" in loc) {
                endpoint = loc.rpc_def.service_name + "." + loc.rpc_def.rpc_name
              }
              return <li key={tr.id}>
                <div className="px-4 py-4 sm:px-6 hover:bg-gray-50" onClick={() => this.setState({selected: tr})}>
                  <div className="flex items-center justify-between">
                    <p className="text-base font-medium text-gray-800 truncate">
                      {endpoint}
                    </p>
                    <div className="ml-2 flex-shrink-0 flex">
                      {tr.root.err === null ? (
                        <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-green-100 text-green-800">
                          Success
                        </span>
                      ) : (
                        <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-red-100 text-red-800">
                          Error
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="mt-2 sm:flex sm:justify-between">
                    <div className="sm:flex">
                      <p className="text-sm font-medium text-indigo-600 truncate flex items-center hover:underline cursor-pointer">
                        <svg className="h-4 w-4 mr-1" fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
                          <polyline points="2 14.308 5.076 14.308 8.154 2 11.231 20.462 14.308 9.692 15.846 14.308 18.924 14.308" />
                          <circle cx="20.462" cy="14.308" r="1.538" />
                        </svg>
                        View Trace
                      </p>
                    </div>
                    <div className="mt-2 flex items-center text-sm text-gray-500 sm:mt-0">
                      <svg className="h-4 w-4 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                      {tr.end_time ? latencyStr(tr.end_time - tr.start_time) : "Unknown"}
                    </div>
                  </div>
                </div>
              </li>
            })}
          </ul>
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

const TraceView: FC<TraceViewProps> = (props) => {
  const tr = props.trace
  const dt = timeToDate(tr.date)!
  const [selected, setSelected] = useState(tr.root)
  const [stack, setStack] = useState<Stack | undefined>(undefined)

  return (
    <section className="bg-white flex-grow flex items-stretch h-full relative">
      <div className="absolute -top-2 -right-2">
        <div className="hover:bg-gray-100 rounded-full p-1 cursor-pointer"
            onClick={() => props.close()}>
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </div>
      </div>

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
            <TraceMap trace={tr} selected={selected} onSelect={setSelected} />
          </div>
        </div>

        <div className="flex flex-col mt-4 relative">
          {stack ? <div className="mr-4">
            <h3 className="text-xl font-semibold mb-2 flex items-center justify-between">
              Stack Trace
              <button className="focus:outline-none hover:text-gray-600" onClick={() => setStack(undefined)}>
                {icons.x("h-5 w-5")}
              </button>
            </h3>
            <StackTrace stack={stack} />
          </div> : <>
            <h3 className="text-xl font-semibold mb-2">Request Tree</h3>
            <SpanList trace={tr} selected={selected} onSelect={setSelected} />
          </>}
        </div>
      </div>

      <div className="flex-shrink-0 w-96 md:w-1/2 p-4 border-l border-gray-100 overflow-scroll">
        <SpanDetail req={selected} trace={tr} onStackTrace={setStack} />
      </div>
    </section>
  )
}