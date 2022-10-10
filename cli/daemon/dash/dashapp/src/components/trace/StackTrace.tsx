import React, { FC, useEffect, useState } from "react";
import { Stack, StackFrame } from "~c/trace/model";
import { icons } from "~c/icons";
import { useConn } from "~lib/ctx";
import { useParams } from "react-router-dom";

const StackTrace: FC<{ stack: Stack }> = (props) => {
  const frames = props.stack.frames;

  return (
    <div className="border-gray-200 bg-gray-100 divide-gray-200 divide-y rounded border text-xs">
      {frames.map((f, i) => (
        <StackFrame key={i} frame={f} expanded={i === 0} last={i === frames.length - 1} />
      ))}
    </div>
  );
};

interface SourceContextResponse {
  lines: string[];
  start: number;
}

const StackFrame: FC<{ frame: StackFrame; expanded: boolean; last: boolean }> = (props) => {
  const frame = props.frame;
  const { appID } = useParams<{ appID: string }>();
  const conn = useConn();
  const [expanded, setExpanded] = useState(props.expanded);
  const [ctx, setCtx] = useState<SourceContextResponse | undefined>(undefined);

  useEffect(() => {
    if (expanded) {
      (async () => {
        const resp = (await conn.request("source-context", {
          appID,
          file: frame.full_file,
          line: frame.line,
        })) as SourceContextResponse;
        setCtx(resp);
      })();
    }
  }, [expanded, frame]);

  const cssProps = { "--line-start": (ctx?.start ?? 1) - 1 } as React.CSSProperties;

  return (
    <div>
      <button
        className="hover:bg-gray-50 group flex w-full cursor-pointer justify-between px-2 py-1 focus:outline-none"
        onClick={() => setExpanded(!expanded)}
      >
        <div>
          <span className="text-gray-700 font-semibold">{frame.short_file}</span>
          {" in "}
          <span className="text-gray-700 font-semibold">{frame.func}</span>
          {" at line "}
          <span className="text-gray-700 font-semibold">{frame.line}</span>
        </div>
        <span className="bg-gray-50 border-gray-200 text-gray-600 group-hover:bg-indigo-600 inline-flex flex-none items-center rounded border group-hover:text-white">
          {expanded ? icons.minus("h-3 w-3 m-0.5") : icons.plus("h-4 w-4")}
        </span>
      </button>
      {expanded && ctx && (
        <pre
          className={`${props.last ? "rounded-b" : ""} code overflow-auto bg-white`}
          style={cssProps}
        >
          {ctx.lines.map((l, i) => (
            <code
              key={i}
              className={ctx.start + i === frame.line ? "bg-indigo-600 text-white" : undefined}
            >
              {l}
            </code>
          ))}
        </pre>
      )}
    </div>
  );
};

export default StackTrace;
