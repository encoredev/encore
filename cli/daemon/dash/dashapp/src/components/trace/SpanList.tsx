import { FunctionComponent, useState } from "react"
import { Request, Trace } from "./model"
import { svcColor } from "./util"
import * as icons from "~c/icons"
import React from "react"

interface Props {
  trace: Trace;
  selected?: Request;
  onSelect?: (req: Request) => void;
}

const SpanList: FunctionComponent<Props> = (props) => {
  const traceDur = props.trace.end_time! - props.trace.start_time
  const [contracted, setContracted] = useState(new Map<string, boolean>())

  let spanCounter = 0
  const renderSpan: (req: Request, level: number, siblings: number[]) => JSX.Element = (req, level, siblings) => {
    const start = Math.round((req.start_time - props.trace.start_time) / traceDur * 100)
    const end = Math.round((req.end_time! - props.trace.start_time) / traceDur * 100)
    const defLoc = props.trace.locations[req.def_loc]

    let svcName = "unknown", rpcName = "Unknown"
    if ("rpc_def" in defLoc) {
      svcName = defLoc.rpc_def.service_name
      rpcName = defLoc.rpc_def.rpc_name
    } else if ("auth_handler_def" in defLoc) {
      svcName = defLoc.auth_handler_def.service_name
      rpcName = defLoc.auth_handler_def.name
    }

    const [color, highlightColor] = svcColor(svcName)
    const sel = props.selected?.id === req.id 

    const select = () => {
      if (props.onSelect) props.onSelect(req)
      contracted.set(req.id, !(contracted.get(req.id) ?? false))
      setContracted(contracted)
    }

    const isContracted = contracted.get(req.id) ?? false
    const showChildren = !isContracted && req.children.length > 0
    spanCounter++
    return <React.Fragment key={req.id}>
      <div className={`flex items-stretch p-4 cursor-pointer ${sel ? "bg-blue-50" : "hover:bg-gray-50"}`} onClick={select}>
        <TreeHint up={level > 0} down={showChildren} siblings={siblings} level={level} />

        {(isContracted && req.children.length > 0) ?
          icons.chevronRight("h-4 w-auto ml-1 mr-0.5") :
          icons.chevronDown("h-4 w-auto ml-1 mr-0.5")
        }

        <div className="flex flex-grow items-center ml-1">
          <div className="w-64 flex-shrink-0 text-xs">
            <div className={`font-semibold ${req.err !== null ? "text-red-700" : "text-gray-800"}`}>
              {svcName}.{rpcName}
            </div>
          </div>
          <div className="flex-grow flex-shrink min-w-0">
            <style>{`
              .spanlist-${spanCounter}       { background-color: ${sel ? highlightColor : color}; }
              .spanlist-${spanCounter}:hover { background-color: ${highlightColor}; }
            `}</style>
            <div className="relative" style={{height: "8px"}}>
              <div className="absolute inset-x-0 bg-gray-200" style={{height: "1px", top: "3px"}} />
              <div key={req.id} className={`absolute inset-y-0 spanlist-${spanCounter}`}
                  style={{
                    left: start+"%", right: (100-end)+"%",
                    minWidth: "2px", // so it at least renders if start === stop
                    borderRadius: "3px",
                  }} />
            </div>
          </div>
        </div>
      </div>

      {showChildren && req.children.map((ch, i) =>
        renderSpan(ch, level+1, siblings.concat(i < (req.children.length-1) ? [level+1] : []))
      )}
    </React.Fragment>
  }

  return (
    <div>
      {props.trace.auth && renderSpan(props.trace.auth, 0, [])}
      {renderSpan(props.trace.root, 0, [])}
    </div>
  )
}

const TreeHint: FunctionComponent<{up: boolean, down: boolean, siblings: number[], level: number}> = (props) => {
  const lvl = props.level
  return <div className="-my-4 relative" style={{width: (lvl*10 + 20)+"px"}}>
    {props.siblings.map(s =>
      <div key={s} className="bg-gray-200 absolute" style={{top: 0, bottom: 0, width: "1px", left: (s*10)+"px"}} />
    )}

    {props.up && <div className="bg-gray-200 absolute" style={{left: (lvl*10)+"px", top: 0, bottom: "50%", width: "1px"}} />}
    <div className="bg-gray-200 absolute" style={{left: (lvl*10)+"px", right: 0, height: "1px", top: "50%"}} />
    {props.down && <div className="bg-gray-200 absolute" style={{width: "1px", left: (lvl*10 + 10)+"px", top: "50%", bottom: 0}} />}
  </div>
}

export default SpanList