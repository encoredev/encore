import React, {FC, useEffect, useState} from "react"
import { Stack, StackFrame } from '~c/trace/model'
import * as icons from "~c/icons"
import { useConn } from "~lib/ctx"
import { useParams } from "react-router-dom"

const StackTrace: FC<{stack: Stack}> = (props) => {
  const frames = props.stack.frames

  return <div className="border border-gray-200 bg-gray-100 rounded divide-y divide-gray-200 text-xs">
    {frames.map((f, i) =>
      <StackFrame key={i} frame={f} expanded={i === 0} last={i === (frames.length-1) }/>
    )}
  </div>
}

interface SourceContextResponse {
  lines: string[];
  start: number;
}

const StackFrame: FC<{frame: StackFrame, expanded: boolean; last: boolean}> = (props) => {
  const frame = props.frame
  const { appID } = useParams<{appID: string}>()
  const conn = useConn()
  const [expanded, setExpanded] = useState(props.expanded)
  const [ctx, setCtx] = useState<SourceContextResponse | undefined>(undefined)
  
  useEffect(() =>  {
    if (expanded) {
      (async () => {
        const resp = await conn.request("source-context", {appID, file: frame.full_file, line: frame.line}) as SourceContextResponse
        setCtx(resp)
      })()
    }
  }, [expanded, frame])

  const cssProps = {"--line-start": (ctx?.start ?? 1) - 1} as React.CSSProperties

  return (
    <div>
      <button className="w-full px-2 py-1 flex justify-between group cursor-pointer hover:bg-gray-50 focus:outline-none"
          onClick={() => setExpanded(!expanded)}>
        <div>
          <span className="font-semibold text-gray-700">{frame.short_file}</span>
          {" in "}
          <span className="font-semibold text-gray-700">{frame.func}</span>
          {" at line "}
          <span className="font-semibold text-gray-700">{frame.line}</span>
        </div>
        <span className="flex-none bg-gray-50 border border-gray-200 rounded inline-flex items-center text-gray-600 group-hover:text-white group-hover:bg-indigo-600">
          {expanded ? icons.minus("h-3 w-3 m-0.5") : icons.plus("h-4 w-4")}
        </span>
      </button>
      {expanded && ctx && <pre className={`${props.last ? "rounded-b" : ""} bg-white code overflow-auto`} style={cssProps}>
        {ctx.lines.map((l, i) => 
          <code key={i} className={(ctx.start + i) === frame.line ? "text-white bg-indigo-600" : undefined}>{l}</code>
        )}
      </pre>}
    </div>
  )
}

export default StackTrace