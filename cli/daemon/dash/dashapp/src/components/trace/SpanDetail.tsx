import { FunctionComponent, useState, useRef, useEffect } from "react"
import { Request, Trace, Event, DBTransaction, DBQuery, TraceExpr, RPCCall, AuthCall } from "./model"
import { latencyStr, svcColor } from "./util"
import * as icons from "~c/icons"
import { decodeBase64, Base64EncodedBytes } from "~lib/base64"
import React from "react"

interface Props {
  trace: Trace;
  req: Request;
}

const SpanDetail: FunctionComponent<Props> = (props) => {
  const req = props.req
  const defLoc = props.trace.locations[req.def_loc]
  const callLoc = req.call_loc !== null ? props.trace.locations[req.call_loc] : null

  const numCalls = req.children.length
  let numQueries = 0
  for (const e of req.events) {
    if (e.type === "DBQuery" ) { numQueries++ }
    else if (e.type === "DBTransaction" ) { numQueries += e.queries.length }
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
      <h2 className="text-2xl font-bold">{svcName}.{rpcName}</h2>
      <div className="text-xs">
        <span className="text-blue-700">
          {defLoc.filepath}:{defLoc.src_line_start}
        </span>
        {callLoc !== null &&
          <span className="text-gray-400">{" "}
            (Called from <span className="text-blue-700">{callLoc.filepath}:{callLoc.src_line_start}</span>)
          </span>
        }
      </div>

      <div className="py-3 grid grid-cols-3 gap-4 border-b border-gray-100">
        <div className="flex items-center text-sm font-light text-gray-400">
          {icons.clock("h-5 w-auto")}
          <span className="font-bold mx-1 text-gray-800">{latencyStr(req.end_time - req.start_time)}</span>
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
      </div>

      <div>
        <div className="mt-6">
          <EventMap trace={props.trace} req={req} />
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
              <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Error</h4>
              <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-red-800 text-sm">{decodeBase64(req.err)}</pre>
            </div>
          ) : (
            <div className="mt-4">
              <h4 className="text-xs font-semibold font-sans text-gray-300 leading-3 tracking-wider uppercase mb-2">Response</h4>
              {req.outputs.length > 0 ? renderData(req.outputs) : <div className="text-gray-700 text-sm">No response data.</div>}
            </div>
          )}
        </>}
      </div>

    </div>
  </>
}

export default SpanDetail

type gdata = {goid: number, start: number, end: number, events: Event[]}

const EventMap: FunctionComponent<{req: Request, trace: Trace}> = (props) => {
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
          <GoroutineDetail key={g.goid} g={g} req={req} trace={props.trace} />
        </div>
      )}
    </div>
  )
}

const GoroutineDetail: FunctionComponent<{g: gdata, req: Request, trace: Trace}> = (props) => {
  const req = props.req
  const reqDur = req.end_time - req.start_time
  const start = Math.round((props.g.start - req.start_time) / reqDur * 100)
  const end = Math.round((props.g.end - req.start_time) / reqDur * 100)
  const g = props.g
  const gdur = g.end - g.start
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
    if (!el) {
      return
    }

    setBarOver(true)
    setHoverObj(obj)
    const spanEl = (ev.target as HTMLElement)
    el.style.marginTop = `calc(${spanEl.offsetTop}px - 40px)`;
    el.style.marginLeft = `calc(${spanEl.offsetLeft}px)`;
  }

  return <>
    <div className="relative" style={{height: lineHeight+"px"}}>
      <div ref={goroutineEl} className={`absolute bg-gray-600`}
        style={{
          borderRadius: "3px",
          height: lineHeight + "px",
          left: start+"%", right: (100-end)+"%",
          minWidth: "1px", // so it at least renders
        }}>

        {g.events.map((ev, i) => {
          const start = Math.round((ev.start_time - g.start) / gdur * 100)
          const end = Math.round((ev.end_time - g.start) / gdur * 100)
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
          }
        })}
      </div>

    </div>
    <div ref={tooltipRef} className="absolute w-full max-w-md pr-2 z-30 transform -translate-x-full"
        style={{paddingRight: "10px" /* extra padding to make it easier to hover into the tooltip */}}
        onMouseEnter={() => setTooltipOver(true)} onMouseLeave={() => setTooltipOver(false)}>
      {(barOver || tooltipOver) && 
        <div className="bg-white w-full p-3 border border-gray-100 rounded-md shadow-lg">
          {hoverObj && "type" in hoverObj && (
            hoverObj.type === "DBQuery" ? <DBQueryTooltip q={hoverObj} trace={props.trace} /> :
            hoverObj.type === "RPCCall" ? <RPCCallTooltip call={hoverObj as RPCCall} req={req} trace={props.trace} /> :
            null)}
        </div>
      }
    </div>
  </>
}

const DBQueryTooltip: FunctionComponent<{q: DBQuery, trace: Trace}> = (props) => {
  const q = props.q
  return <div>
    <h3 className="flex items-center text-gray-800 font-bold text-lg">
      {icons.database("h-8 w-auto text-gray-400 mr-2")}
      DB Query
      <div className="ml-auto text-sm font-normal text-gray-500">{latencyStr(q.end_time - q.start_time)}</div>
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

const RPCCallTooltip: FunctionComponent<{call: RPCCall, req: Request, trace: Trace}> = (props) => {
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
      <div className="ml-auto text-sm font-normal text-gray-500">{latencyStr(c.end_time - c.start_time)}</div>
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

const renderData = (data: Base64EncodedBytes[]) => {
  const json = JSON.parse(decodeBase64(data[0]))
  return <pre className="rounded overflow-auto border border-gray-200 p-2 bg-gray-100 text-gray-800 text-sm">{JSON.stringify(json, undefined, "  ")}</pre>
}