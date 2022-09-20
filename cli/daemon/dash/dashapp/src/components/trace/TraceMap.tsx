import React, { FunctionComponent, useEffect, useState } from "react";
import { Request, Trace } from "./model";
import { svcColor } from "./util";

interface Props {
  trace: Trace;
  selected?: Request;
  onSelect?: (req: Request) => void;
}

const TraceMap: FunctionComponent<Props> = (props) => {
  const root = props.trace.root;
  const traceStart = props.trace.start_time;
  const traceDur = props.trace.end_time! - traceStart;
  const lineHeight = 8;
  const lineGap = 2;

  const renderSpan = (req: Request, line: number) => {
    const start = Math.round(((req.start_time - traceStart) / traceDur) * 100);
    const end = Math.round(((req.end_time! - traceStart) / traceDur) * 100);
    const defLoc = props.trace.locations[req.def_loc];
    let svcName = "unknown";
    if ("rpc_def" in defLoc) {
      svcName = defLoc.rpc_def.service_name;
    }
    const [color, highlightColor] = svcColor(svcName);
    const sel = props.selected?.id === req.id;
    const select = () => props.onSelect && props.onSelect(req);

    return (
      <React.Fragment key={req.id}>
        <style>{`
          .span {
            cursor: pointer;
            background-color: ${sel ? highlightColor : color};
          }
          .span:hover {
            background-color: ${highlightColor};
          }
        `}</style>
        <div
          key={req.id}
          className={`span absolute border ${
            sel ? "border-black border-opacity-[30%]" : "border-transparent"
          }`}
          onClick={select}
          data-span={req.id}
          style={
            {
              "--data-start": "" + start,
              "--data-end": "" + end,
              "--data-line": line,
              top: line * (lineHeight + lineGap) + "px",
              height: lineHeight + "px",
              left: start + "%",
              right: 100 - end + "%",
              minWidth: "1px", // so it at least renders if start === stop
            } as any
          }
        />
      </React.Fragment>
    );
  };

  let [lines, setLines] = useState<Request[][]>([]);
  let roots: Request[] = [];
  if (props.trace.root !== null) {
    roots.push(props.trace.root);
  }
  if (props.trace.auth !== null) {
    roots.push(props.trace.auth);
  }
  useEffect(() => setLines(buildTraceMap(roots)), [props.trace]);

  return (
    <div
      className="relative"
      style={{ height: lines.length * (lineHeight + lineGap) - lineGap + "px" }}
    >
      {lines.map((line, i) => line.map((span, j) => renderSpan(span, i)))}
    </div>
  );
};

export default TraceMap;

// buildTraceMaps computes the layout for the trace map.
// The result is a two-dimensional array, where the outer array consists of lines
// and the inner array is a list of non-overlapping spans in that line.
function buildTraceMap(roots: Request[]): Request[][] {
  // Layout the spans on the trace map.
  // For a given span, look for available space in lines through
  // a naive loop over the spans in lines with idx > x, where x
  // is the parent's line index.
  const lines: Request[][] = [];
  const queue = roots.map((r) => {
    return { span: r, minLine: 0 };
  });
  while (queue.length > 0) {
    const { span, minLine } = queue.shift()!;
    let spanLine: number | undefined = undefined;
    for (let i = minLine; i < lines.length; i++) {
      const line = lines[i];
      const nl = line.length;

      // Find an available gap in the line.
      for (let j = 0; j < nl; j++) {
        const start = line[j].start_time;
        const end = line[j].end_time!;
        if (
          (j === 0 && span.end_time! <= start) || // before first
          (j === nl - 1 && span.start_time >= end) || // after last
          (j > 0 &&
            j < nl - 1 &&
            span.start_time >= end &&
            span.end_time! <= line[j + 1].start_time) // in gap between spans
        ) {
          spanLine = i;
          line.splice(j, 0, span);
          break;
        }
      }
      if (spanLine !== undefined) {
        break;
      }
    }

    if (spanLine === undefined) {
      // Add a new line to accommodate it
      lines.push([span]);
      spanLine = lines.length - 1;
    }

    // Add all child spans to the queue
    for (const child of span.children) {
      queue.push({ span: child, minLine: spanLine + 1 });
    }
  }

  return lines;
}
