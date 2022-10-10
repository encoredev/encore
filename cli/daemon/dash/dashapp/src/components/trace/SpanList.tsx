import React, { FC, FunctionComponent, useState } from "react";
import { Request, Trace } from "./model";
import { svcColor } from "./util";
import { Icon, icons } from "~c/icons";

interface Props {
  trace: Trace;
  selected: Request | undefined;
  onSelect: (req: Request) => void;
}

const SpanList: FunctionComponent<Props> = (props) => {
  return (
    <div>
      {props.trace.auth !== null && (
        <SpanRow
          trace={props.trace}
          req={props.trace.auth}
          level={0}
          siblings={[]}
          selected={props.selected}
          onSelect={props.onSelect}
        />
      )}
      {props.trace.root !== null && (
        <SpanRow
          trace={props.trace}
          req={props.trace.root}
          level={0}
          siblings={[]}
          selected={props.selected}
          onSelect={props.onSelect}
        />
      )}
    </div>
  );
};

const SpanRow: FC<{
  trace: Trace;
  req: Request;
  level: number;
  siblings: number[];
  selected: Request | undefined;
  onSelect: (r: Request) => void;
}> = ({ trace, req, level, siblings, selected, onSelect }) => {
  const [expanded, setExpanded] = useState(true);
  const traceDur = trace.end_time! - trace.start_time;
  const start = Math.round(((req.start_time - trace.start_time) / traceDur) * 100);
  const end = Math.round(((req.end_time! - trace.start_time) / traceDur) * 100);
  const defLoc = trace.locations[req.def_loc];

  let svcName = "unknown",
    rpcName = "Unknown";
  let icon: Icon | undefined;
  let type = "Unknown Request";
  if ("rpc_def" in defLoc) {
    svcName = defLoc.rpc_def.service_name;
    rpcName = defLoc.rpc_def.rpc_name;
    icon = icons.logout;
    type = "API Call";
  } else if ("auth_handler_def" in defLoc) {
    svcName = defLoc.auth_handler_def.service_name;
    rpcName = defLoc.auth_handler_def.name;
    icon = icons.shield;
    type = "Auth Call";
  } else if ("pubsub_subscriber" in defLoc) {
    svcName = defLoc.pubsub_subscriber.topic_name;
    rpcName = defLoc.pubsub_subscriber.subscriber_name;
    icon = icons.arrowsExpand;
    type = "PubSub Message Received";
  }

  const [color, highlightColor] = svcColor(svcName);
  const sel = selected?.id === req.id;
  const select = () => {
    onSelect(req);
    setExpanded(!expanded);
  };

  const showChildren = expanded && req.children.length > 0;
  return (
    <>
      <div
        className={`flex cursor-pointer select-none items-stretch p-4 ${
          sel ? "bg-black bg-opacity-[5%]" : ""
        }`}
        onClick={select}
      >
        <TreeHint up={level > 0} down={showChildren} siblings={siblings} level={level} />

        {expanded && req.children.length > 0
          ? icons.chevronRight("h-3 w-auto ml-1 mr-0.5")
          : icons.chevronDown("h-3 w-auto ml-1 mr-0.5")}

        <div className="ml-1 flex flex-grow items-center">
          <div className="min-w-0 flex-1 truncate text-xs">
            <div className={`font-semibold ${req.err !== null ? "text-red" : "text-black"}`}>
              {icon ? icon("h-3 w-3 mr-1 inline-block", type) : undefined}
              {svcName}.{rpcName}
            </div>
          </div>
          <div className="w-64 flex-none">
            <style>{`
              .spanlist-${req.id}       { background-color: ${sel ? highlightColor : color}; }
              .spanlist-${req.id}:hover { background-color: ${highlightColor}; }
            `}</style>
            <div className="relative" style={{ height: "8px" }}>
              <div
                className="absolute inset-x-0 bg-lightgray"
                style={{ height: "1px", top: "3px" }}
              />
              <div
                key={req.id}
                className={`absolute inset-y-0 spanlist-${req.id}`}
                style={{
                  left: start + "%",
                  right: 100 - end + "%",
                  minWidth: "2px", // so it at least renders if start === stop
                }}
              />
            </div>
          </div>
        </div>
      </div>

      {showChildren &&
        req.children.map((ch, i) => (
          <SpanRow
            key={i}
            trace={trace}
            req={ch}
            level={level + 1}
            selected={selected}
            onSelect={onSelect}
            siblings={siblings.concat(i < req.children.length - 1 ? [level + 1] : [])}
          />
        ))}
    </>
  );
};

const TreeHint: FunctionComponent<{
  up: boolean;
  down: boolean;
  siblings: number[];
  level: number;
}> = (props) => {
  const lvl = props.level;
  return (
    <div className="relative -my-4" style={{ width: lvl * 10 + 20 + "px" }}>
      {props.siblings.map((s) => (
        <div
          key={s}
          className="absolute bg-lightgray"
          style={{ top: 0, bottom: 0, width: "1px", left: s * 10 + "px" }}
        />
      ))}

      {props.up && (
        <div
          className="absolute bg-lightgray"
          style={{ left: lvl * 10 + "px", top: 0, bottom: "50%", width: "1px" }}
        />
      )}
      <div
        className="absolute bg-lightgray"
        style={{ left: lvl * 10 + "px", right: 0, height: "1px", top: "50%" }}
      />
      {props.down && (
        <div
          className="absolute bg-lightgray"
          style={{
            width: "1px",
            left: lvl * 10 + 10 + "px",
            top: "50%",
            bottom: 0,
          }}
        />
      )}
    </div>
  );
};

export default SpanList;
