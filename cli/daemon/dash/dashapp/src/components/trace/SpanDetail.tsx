import { Duration } from "luxon"
import React, { FunctionComponent, useRef, useState } from "react"
import * as icons from "~c/icons"
import { Base64EncodedBytes, decodeBase64 } from "~lib/base64"
import { timeToDate } from "~lib/time"
import { DBQuery, Event, HTTPCall, LogMessage, Request, RPCCall, Stack, Trace } from "./model"
import { latencyStr, svcColor } from "./util"

interface Props {
  trace: Trace;
  req: Request;
  onStackTrace: (s: Stack) => void;
}

const SpanDetail: FunctionComponent<Props> = (props) => {
  const req = props.req
  const tr = props.trace
  const defLoc = props.trace.locations[req.def_loc]
  const call = findCall(props.trace, req.id)

  const numCalls = req.children.length
  let numQueries = 0
  let logs: LogMessage[] = []
  for (const e of req.events) {
    if (e.type === "DBQuery" ) { numQueries++ }
    else if (e.type === "DBTransaction" ) { numQueries += e.queries.length }
    else if (e.type === "LogMessage") { logs.push(e) }
  }

  let svcName = "unknown", rpcName = "Unknown"
  if ("rpc_def" in defLoc) {
    svcName = defLoc.rpc_def.service_name
    rpcName = defLoc.rpc_def.rpc_name
  } else if ("auth_handler_def" in defLoc) {
    svcName = defLoc.auth_handler_def.service_name
    rpcName = defLoc.auth_handler_def.name
  }

  return <>
    <div>
      <h2 className="text-2xl font-bold flex items-center">
        {svcName}.{rpcName}
        {call && 
          <button className="text-gray-600 hover:text-indigo-600 focus:outline-none"
            onClick={() => props.onStackTrace(call.stack)}>{icons.stackTrace("m-1 h-4 w-auto")}</button>
        }
      </h2>
      <div className="text-xs">
        <span className="text-blue-700">
          {defLoc.filepath}:{defLoc.src_line_start}
        </span>
      </div>

      <div className="py-3 grid grid-cols-4 gap-4 border-b border-gray-100">
        <div className="flex items-center text-sm font-light text-gray-400">
          {icons.clock("h-5 w-auto")}
          <span className="font-bold mx-1 text-gray-800">{req.end_time ? latencyStr(req.end_time - req.start_time) : "Unknown"}</span>
          Duration
        </div>

        <div className="flex items-center text-sm font-light text-gray-400">
          {icons.logout("h-5 w-auto")}
          <span className="font-bold mx-1 text-gray-800">{numCalls}</span>
          API Call{numCalls !== 1 ? "s" : ""}
        </div>

        <div className="flex items-center text-sm font-light text-gray-400">
          {icons.database("h-5 w-auto")}
          <span className="font-bold mx-1 text-gray-800">{numQueries}</span>
          DB Quer{numQueries !== 1 ? "ies" : "y"}
        </div>

        <div className="flex items-center text-sm font-light text-gray-400">
          {icons.menuAlt2("h-5 w-auto")}
          <span className="font-bold mx-1 text-gray-800">{logs.length}</span>
          Log Line{numQueries !== 1 ? "s" : ""}
        </div>
      </div>

      <div>
        <div className="mt-6">
          <EventMap trace={props.trace} req={req} onStackTrace={props.onStackTrace} />
        </div>

        {req.type === "AUTH" ? (
          req.err !== null ? (
            <div className="mt-4">
              <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Error</h4>
              <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-red-800 text-sm">{decodeBase64(req.err)}</pre>
            </div>
          ) : (
            <>
              <div className="mt-6">
                <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">User ID</h4>
                {renderData([req.outputs[0]])}
              </div>
              {req.outputs.length > 1 &&
                <div className="mt-4">
                  <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">User Data</h4>
                  {renderData([req.outputs[1]])}
                </div>
              }
            </>
          )
        ) : <>
          <div className="mt-6">
            <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Request</h4>
            {req.inputs.length > 0 ? renderData(req.inputs) : <div className="text-gray-700 text-sm">No request data.</div>}
          </div>
          {req.err !== null ? (
            <div className="mt-4">
              <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2 flex items-center">
                Error <button className="text-gray-600 hover:text-indigo-600 focus:outline-none ml-1"
                  onClick={() => props.onStackTrace(req.err_stack!)}>{icons.stackTrace("m-1 h-4 w-auto")}</button>
              </h4>
              <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-red-800 text-sm">{decodeBase64(req.err)}</pre>
            </div>
          ) : (
            <div className="mt-4">
              <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Response</h4>
              {req.outputs.length > 0 ? renderData(req.outputs) : <div className="text-gray-700 text-sm">No response data.</div>}
            </div>
          )}
        </>}

        {logs.length > 0 &&
          <div className="mt-6">
            <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Logs</h4>
            <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-gray-800 text-xs">
              {logs.map((log, i) => renderLog(tr, log, i, props.onStackTrace))}
            </pre>
          </div>
        }
      </div>

    </div>
  </>
}

export default SpanDetail

type gdata = {goid: number, start: number, end: number | undefined, events: Event[]}

const EventMap: FunctionComponent<{req: Request, trace: Trace, onStackTrace: (s: Stack) => void}> = (props) => {
  const req = props.req

  // Compute the list of interesting goroutines
  const gmap: {[key: number]: gdata} = {
    [req.goid]: {
      goid: req.goid,
      start: req.start_time,
      end: req.end_time,
      events: [],
    }
  }
  const gnums: number[] = [req.goid]

  for (const ev of req.events) {
    if (ev.type === "Goroutine") {
      gmap[ev.goid] = { goid: ev.goid, start: ev.start_time, end: ev.end_time, events: [] }
      gnums.push(ev.goid)
    } else if (ev.type === "DBTransaction") {
      let g = gmap[ev.goid]
      g.events = g.events.concat(ev.queries)
    } else {
      gmap[ev.goid].events.push(ev)
    }
  }

  const lines = gnums.map(n => gmap[n]).filter(g => (g.events.length > 0 || g.goid === req.goid))
  return (
    <div>
      <div className="flex items-center text-xs font-light text-gray-400 mb-1">
        {icons.chip("h-4 w-auto")}
        <span className="font-bold mx-1 text-gray-800">{lines.length}</span>
        Goroutine{lines.length !== 1 ? "s" : ""}
      </div>
      {lines.map((g, i) => 
        <div key={g.goid} className={i > 0 ? "mt-1" : ""}>
          <GoroutineDetail key={g.goid} g={g} req={req} trace={props.trace} onStackTrace={props.onStackTrace} />
        </div>
      )}
    </div>
  )
}

const GoroutineDetail: FunctionComponent<{g: gdata, req: Request, trace: Trace, onStackTrace: (s: Stack) => void}> = (props) => {
  const req = props.req
  const reqDur = req.end_time! - req.start_time
  const start = Math.round((props.g.start - req.start_time) / reqDur * 100)
  const end = Math.round((props.g.end! - req.start_time) / reqDur * 100)
  const g = props.g
  const gdur = g.end! - g.start
  const lineHeight = 18

  const tooltipRef = useRef<HTMLDivElement>(null)
  const goroutineEl = useRef<HTMLDivElement>(null)
  const [hoverObj, setHoverObj] = useState<Request | Event | null>(null)
  const [barOver, setBarOver] = useState(false)
  const [tooltipOver, setTooltipOver] = useState(false)

  const setHover = (ev: React.MouseEvent, obj: Request | Event | null) => {
    if (obj === null) {
      setBarOver(false)
      return
    }

    const el = tooltipRef.current
    const gel = goroutineEl.current
    if (!el || !gel) {
      return
    }

    setBarOver(true)
    setHoverObj(obj)
    const spanEl = (ev.target as HTMLElement)
    el.style.marginTop = `calc(${spanEl.offsetTop}px - 40px)`;
    el.style.transform = `translateX(calc(-100% + ${gel.offsetLeft}px + ${spanEl.offsetLeft}px))`
  }

  const barEvents: (DBQuery | RPCCall | HTTPCall)[] = g.events.filter(e => e.type === "DBQuery" || e.type === "RPCCall" || e.type === "HTTPCall") as any

  return <>
    <div className="relative" style={{height: lineHeight+"px"}}>
      <div ref={goroutineEl} className={`absolute bg-gray-600`}
        style={{
          borderRadius: "3px",
          height: lineHeight + "px",
          left: start+"%", right: (100-end)+"%",
          minWidth: "1px", // so it at least renders
        }}>

        {barEvents.map((ev, i) => {
          const start = Math.round((ev.start_time - g.start) / gdur * 100)
          const end = Math.round((ev.end_time! - g.start) / gdur * 100)
          const clsid = `ev-${req.id}-${g.goid}-${i}`

          if (ev.type === "DBQuery") {
            const [color, highlightColor] = svcColor(ev.txid !== null ? ("tx:"+ev.txid) : ("query:"+ev.start_time))
            return <React.Fragment key={i}>
              <style>{`
                .${clsid}       { background-color: ${highlightColor}; }
                .${clsid}:hover { background-color: ${color}; }
              `}</style>
              <div className={`absolute ${clsid}`}
                onMouseEnter={(e) => setHover(e, ev)}
                onMouseLeave={(e) => setHover(e, null)}
                style={{
                  borderRadius: "3px",
                  top: "3px", bottom: "3px",
                  left: start+"%", right: (100-end)+"%",
                  minWidth: "1px" // so it at least renders if start === stop
                }} />
              </React.Fragment>
          } else if (ev.type === "RPCCall") {
            const defLoc = props.trace.locations[ev.def_loc]
            let svcName = "unknown"
            if ("rpc_def" in defLoc) {
              svcName = defLoc.rpc_def.service_name
            }
            const [color, highlightColor] = svcColor(svcName)
            return <React.Fragment key={i}>
              <style>{`
                .${clsid}       { background-color: ${highlightColor}; }
                .${clsid}:hover { background-color: ${color}; }
              `}</style>
              <div className={`absolute ${clsid}`}
                onMouseEnter={(e) => setHover(e, ev)}
                onMouseLeave={(e) => setHover(e, null)}
                style={{
                  borderRadius: "3px",
                  top: "3px", bottom: "3px",
                  left: start+"%", right: (100-end)+"%",
                  minWidth: "1px" // so it at least renders if start === stop
                }}
              />
            </React.Fragment>
          } else if (ev.type === "HTTPCall") {
            const [color, highlightColor] = svcColor(ev.url)
            return <React.Fragment key={i}>
              <style>{`
                .${clsid}       { background-color: ${highlightColor}; }
                .${clsid}:hover { background-color: ${color}; }
              `}</style>
              <div className={`absolute ${clsid}`}
                onMouseEnter={(e) => setHover(e, ev)}
                onMouseLeave={(e) => setHover(e, null)}
                style={{
                  borderRadius: "3px",
                  top: "3px", bottom: "3px",
                  left: start+"%", right: (100-end)+"%",
                  minWidth: "1px" // so it at least renders if start === stop
                }}
              />
            </React.Fragment>
          }
        })}
      </div>

    </div>
    <div ref={tooltipRef} className="absolute w-full max-w-md pr-2 z-40"
        style={{paddingRight: "10px" /* extra padding to make it easier to hover into the tooltip */}}
        onMouseEnter={() => setTooltipOver(true)} onMouseLeave={() => setTooltipOver(false)}>
      {(barOver || tooltipOver) && 
        <div className="bg-white w-full p-3 border border-gray-100 rounded-md shadow-lg overflow-auto">
          {hoverObj && "type" in hoverObj && (
            hoverObj.type === "DBQuery" ? <DBQueryTooltip q={hoverObj} trace={props.trace} onStackTrace={props.onStackTrace} /> :
            hoverObj.type === "RPCCall" ? <RPCCallTooltip call={hoverObj as RPCCall} req={req} trace={props.trace} onStackTrace={props.onStackTrace} /> :
            hoverObj.type === "HTTPCall" ? <HTTPCallTooltip call={hoverObj as HTTPCall} req={req} trace={props.trace} /> :
            null)}
        </div>
      }
    </div>
  </>
}

const DBQueryTooltip: FunctionComponent<{q: DBQuery, trace: Trace, onStackTrace: (s: Stack) => void}> = (props) => {
  const q = props.q
  return <div>
    <h3 className="flex items-center text-gray-800 font-bold text-lg">
      {icons.database("h-8 w-auto text-gray-400 mr-2")}
      DB Query
      <div className="ml-auto text-sm font-normal text-gray-500 flex items-center">
        {q.end_time ? latencyStr(q.end_time - q.start_time) : "Unknown"}
        <button className="text-gray-600 hover:text-indigo-600 focus:outline-none -mr-1"
          onClick={() => props.onStackTrace(q.stack)}>{icons.stackTrace("m-1 h-4 w-auto")}</button>
      </div>
    </h3>

    <div className="mt-4">
      <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Query</h4>
      {q.html_query !== null ? (
        <pre className="rounded overflow-auto border border-gray-200 text-sm p-2"
            dangerouslySetInnerHTML={{__html: decodeBase64(q.html_query)}} />
      ) : (
        <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-gray-800 text-sm">
          {decodeBase64(q.query)}
        </pre>
      )}
    </div>

    <div className="mt-4">
      <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Error</h4>
      {q.err !== null ? (
        <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-gray-800 text-sm">
          {decodeBase64(q.err)}
        </pre>
      ) : (
        <div className="text-gray-700 text-sm">Completed successfully.</div>
      )}
    </div>

  </div>
}

const RPCCallTooltip: FunctionComponent<{call: RPCCall, req: Request, trace: Trace; onStackTrace: (s: Stack) => void}> = (props) => {
  const c = props.call
  const target = props.req.children.find(r => r.id === c.req_id)
  const defLoc = props.trace.locations[c.def_loc]
  let endpoint: string | null = null
  if ("rpc_def" in defLoc) {
    endpoint = `${defLoc.rpc_def.service_name}.${defLoc.rpc_def.rpc_name}`
  }

  return <div>
    <h3 className="flex items-center text-gray-800 font-bold text-lg">
      {icons.logout("h-8 w-auto text-gray-400 mr-2")}
      API Call
      {endpoint !== null ? (
        <span>: {endpoint}</span>
      ) : (
        <span className="italic text-sm text-gray-500">Unknown Endpoint</span>
      )}
      <div className="ml-auto text-sm font-normal text-gray-500 flex items-center">
        {c.end_time ? latencyStr(c.end_time - c.start_time) : "Unknown"}
        <button className="text-gray-600 hover:text-indigo-600 focus:outline-none -mr-1"
          onClick={() => props.onStackTrace(c.stack)}>{icons.stackTrace("m-1 h-4 w-auto")}</button>
      </div>
    </h3>

    <div className="mt-4">
      <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Request</h4>
      {target !== undefined ? (
        target.inputs.length > 0 ? renderData(target.inputs) : <div className="text-gray-700 text-sm">No request data.</div>
      ) : <div className="text-gray-700 text-sm">Not captured.</div>
      }
    </div>

    <div className="mt-4">
      <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Response</h4>
      {target !== undefined ? (
        target.outputs.length > 0 ? renderData(target.outputs) : <div className="text-gray-700 text-sm">No response data.</div>
      ) : <div className="text-gray-700 text-sm">Not captured.</div>
      }
    </div>

    <div className="mt-4">
      <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Error</h4>
      {c.err !== null ? (
        <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-gray-800 text-sm">
          {decodeBase64(c.err)}
        </pre>
      ) : (
        <div className="text-gray-700 text-sm">Completed successfully.</div>
      )}
    </div>

  </div>
}

const HTTPCallTooltip: FunctionComponent<{call: HTTPCall, req: Request, trace: Trace}> = ({call, req, trace}) => {
  const m = call.metrics
  return <div>
    <h3 className="flex items-center text-gray-800 font-bold text-lg">
      {icons.logout("h-8 w-auto text-gray-400 mr-2")}
      HTTP {call.method} {call.host}{call.path}
      <div className="ml-auto text-sm font-normal text-gray-500">{call.end_time ? latencyStr(call.end_time - call.start_time) : "Unknown"}</div>
    </h3>

    <div className="mt-4">
      <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">URL</h4>
      <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-gray-800 text-sm">{call.url}</pre>
    </div>

    <div className="mt-4">
      <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Response</h4>
      {call.end_time !== -1 ? (
        <div className="text-gray-700 text-sm">HTTP {call.status_code}</div>
      ) : (
        <div className="text-gray-700 text-sm">No response recorded.</div>
      )}
    </div>

    <div className="mt-4">
      <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Error</h4>
      {call.err !== null ? (
        <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-gray-800 text-sm">
          {decodeBase64(call.err)}
        </pre>
      ) : (
        <div className="text-gray-700 text-sm">Completed successfully.</div>
      )}
    </div>

    <div className="mt-4">
      <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Timeline</h4>
      <div className="text-gray-600 text-xs inline-grid grid-cols-2">
        {m.conn_reused ? <>
          <span>Reused Connection:</span> <span className="text-right">Yes</span>
        </> : <>
          {m.dns_done && <><span>DNS Lookup:</span> <span className="text-right">{latencyStr(m.dns_done - call.start_time)}</span></>}
          {m.tls_handshake_done && <><span>TLS Handshake:</span> <span className="text-right">{latencyStr(m.tls_handshake_done - (m.dns_done ?? call.start_time))}</span></>}
        </>}
        {m.wrote_request && <><span>Wrote Request:</span> <span className="text-right">{latencyStr(m.wrote_request - (m.tls_handshake_done ?? m.got_conn ?? call.start_time))}</span></>}
        {m.first_response && <><span>Response Start:</span> <span className="text-right">{latencyStr(m.first_response - (m.wrote_headers ?? m.got_conn ?? call.start_time))}</span></>}
      </div>
    </div>

  </div>
}

const renderData = (data: Base64EncodedBytes[]) => {
  const raw = decodeBase64(data[0])
  let pretty = raw
  try {
    const json = JSON.parse(decodeBase64(data[0]))
    pretty = JSON.stringify(json, undefined, "  ")
  } catch(err) { /* do nothing */ }
  return <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-gray-800 text-sm">{pretty}</pre>
}

const renderLog = (tr: Trace, log: LogMessage, key: any, onStackTrace: (s: Stack) => void) => {
  let dt = timeToDate(tr.date)!
  const ms = (log.time - tr.start_time)/1000
  dt = dt.plus(Duration.fromMillis(ms))
  return <div key={key} className="flex items-center gap-x-1.5">
    <button className="-ml-2 -mr-1 text-gray-600 hover:text-indigo-600 focus:outline-none"
      onClick={() => onStackTrace(log.stack)}>{icons.stackTrace("m-1 h-4 w-auto")}</button>
    <span className="text-gray-400">{dt.toFormat("HH:mm:ss.SSS")}</span>
    {
      log.level === "DEBUG" ? <span className="text-yellow-500">DBG</span> :
      log.level === "INFO" ? <span className="text-green-500">INF</span> :
      <span className="text-red-500">ERR</span>
    }
    {log.msg}
    {log.fields.map((f, i) =>
      <span className="inline-flex items-center">
        {f.stack ? <> 
          <button className="text-red-800 hover:text-red-600 focus:outline-none"
            onClick={() => onStackTrace(f.stack!)}>{icons.stackTrace("h-4 w-auto")}</button>
          <span className="text-red-600">{f.key}</span>
          <span className="text-gray-400">=</span>
          <span className="text-red-600">{f.value}</span>
        </> : <>
          <span className="text-blue-600">{f.key}</span>
          <span className="text-gray-400">=</span>
          {f.value}
        </>}
      </span>
    )}
  </div>
}

function findCall(tr: Trace, id: string): RPCCall | undefined {
  const queue = [tr.root]
  while (queue.length > 0) {
    const req = queue.shift()!
    for (const e of req.events) {
      if (e.type === "RPCCall" && e.req_id === id) {
        return e
      }
    }
    queue.push(...req.children)
  }
  return undefined
}