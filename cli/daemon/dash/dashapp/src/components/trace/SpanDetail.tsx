import { ModeSpec, ModeSpecOptions } from "codemirror";
import { DateTime, Duration } from "luxon";
import React, {
  CSSProperties,
  FC,
  FunctionComponent,
  PropsWithChildren,
  useMemo,
  useRef,
  useState,
} from "react";
import { JSONTree } from "react-json-tree";
import CM from "~c/api/cm/CM";
import { Icon, icons } from "~c/icons";
import { Base64EncodedBytes, decodeBase64 } from "~lib/base64";
import { timeToDate } from "~lib/time";
import {
  CacheOp,
  CacheResult,
  DBQuery,
  Event,
  HTTPCall,
  LogMessage,
  PubSubPublish,
  Request,
  RPCCall,
  Stack,
  Trace,
} from "./model";
import { idxColor, latencyStr } from "./util";
import { copyToClipboard } from "~lib/clipboard";
import { ClipboardDocumentCheckIcon, ClipboardDocumentIcon } from "@heroicons/react/24/outline";
import { CircularPropsPassedThroughJSONTree } from "react-json-tree/lib/types/types";

interface Props {
  trace: Trace;
  req: Request;
  onStackTrace: (s: Stack) => void;
}

const SpanDetail: FunctionComponent<Props> = (props) => {
  const [expandLogs, setExpandLogs] = useState<boolean>(false);
  const req = props.req;
  const tr = props.trace;
  const defLoc = props.trace.locations[req.def_loc];
  const call = findCall(props.trace, req.id);

  const numCalls = req.children.length;
  let numQueries = 0;
  let logs: LogMessage[] = [];
  let publishedMessages: PubSubPublish[] = [];
  for (const e of req.events) {
    if (e.type === "DBQuery") {
      numQueries++;
    } else if (e.type === "DBTransaction") {
      numQueries += e.queries.length;
    } else if (e.type === "LogMessage") {
      logs.push(e);
    } else if (e.type === "PubSubPublish") {
      publishedMessages.push(e);
    }
  }

  let svcName = "unknown",
    rpcName = "Unknown";
  let icon: Icon = icons.exclamation;
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

  const logsRef = useRef<HTMLDivElement>(null);
  const scrollToRef = (ref: React.MutableRefObject<HTMLDivElement | null>) => {
    if (ref.current && ref.current.scrollIntoView) {
      ref.current.scrollIntoView();
    }
  };

  return (
    <>
      <div className="flex h-full flex-col">
        <h2 className="text-2xl font-semibold">
          {icon("h-5 w-5 mr-2 inline-block", type)}
          {svcName}.{rpcName}
          {call && (
            <button className="focus:outline-none" onClick={() => props.onStackTrace(call.stack)}>
              {icons.stackTrace("m-1 h-4 w-auto")}
            </button>
          )}
        </h2>
        <div className="text-xs">
          <span>
            {defLoc.filepath}:{defLoc.src_line_start}
          </span>
        </div>

        <div className="wrap flex w-full flex-row flex-wrap py-3 [&>*]:min-w-[150px] [&>*]:basis-1/5 [&>*]:pb-2">
          <div className="body-sm flex items-center">
            <div>{icons.clock("h-5 w-auto")}</div>
            <span className="mx-1 font-semibold">
              {req.end_time ? latencyStr(req.end_time - req.start_time) : "Unknown"}
            </span>
            Duration
          </div>

          <div className="body-sm flex items-center">
            <div>{icons.logout("h-5 w-auto")}</div>
            <span className="text-gray-800 mx-1 font-semibold">{numCalls}</span>
            API Call{numCalls !== 1 ? "s" : ""}
          </div>

          <div className="body-sm flex items-center">
            <div>{icons.database("h-5 w-auto")}</div>
            <span className="text-gray-800 mx-1 font-semibold">{numQueries}</span>
            DB Quer{numQueries !== 1 ? "ies" : "y"}
          </div>

          <div className="body-sm flex items-center">
            <div>{icons.arrowsExpand("h-5 w-auto")}</div>
            <span className="text-gray-800 mx-1 font-semibold">{publishedMessages.length}</span>
            Publish{publishedMessages.length !== 1 ? "es" : ""}
          </div>

          <div
            className={`body-sm flex items-center ${logs.length ? "cursor-pointer" : ""}`}
            onClick={() => scrollToRef(logsRef)}
          >
            <div>{icons.menuAlt2("h-5 w-auto")}</div>
            <span className="text-gray-800 mx-1 font-semibold">{logs.length}</span>
            Log Line{logs.length !== 1 ? "s" : ""}
          </div>
        </div>

        <div className="h-full overflow-auto">
          <div className="mt-6">
            <EventMap trace={props.trace} req={req} onStackTrace={props.onStackTrace} />
          </div>

          <NewRequestInfo req={req} trace={tr} onStackTrace={props.onStackTrace} />

          {logs.length > 0 && (
            <div className="mt-6">
              <div className="flex items-baseline justify-between">
                <h4 className="mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
                  Logs
                </h4>
                <span
                  className="flex cursor-pointer select-none items-center text-xs normal-case underline"
                  onClick={() => setExpandLogs(!expandLogs)}
                >
                  {expandLogs ? icons.collapseAll("h-4 w-4") : icons.expandAll("h-4 w-4")}
                </span>
              </div>
              <CodeBox className="overflow-auto">
                {logs.map((log, i) => renderLogWithJSON(tr, log, i, expandLogs))}
              </CodeBox>
            </div>
          )}
        </div>
      </div>
    </>
  );
};

export default SpanDetail;

const NewRequestInfo: FC<{ req: Request; trace: Trace; onStackTrace: (s: Stack) => void }> = ({
  req,
  trace,
  onStackTrace,
}) => {
  const svc = trace.meta.svcs.find((s) => s.name === req.svc_name);
  const rpc = svc?.rpcs.find((r) => r.name === req.rpc_name);
  const isRaw = rpc?.proto === "RAW";

  return req.type === "AUTH" ? (
    req.err !== null ? (
      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Error
        </h4>
        <CodeBox error>{decodeBase64(req.err)}</CodeBox>
      </div>
    ) : (
      <>
        <div className="mt-6">
          <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
            User ID
          </h4>
          {req.user_id}
        </div>
        {req.response_payload && (
          <div className="mt-4">
            <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
              User Data
            </h4>
            <CodeBox>
              <PayloadViewer payload={req.response_payload} />
            </CodeBox>
          </div>
        )}
      </>
    )
  ) : req.type === "PUBSUB_MSG" ? (
    <>
      <div className="mt-6">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Message ID
        </h4>
        <div className="text-gray-700 text-sm">{req.msg_id ?? "<unknown>"}</div>
      </div>
      <div className="grid grid-cols-2">
        <div className="mt-6">
          <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
            Delivery Attempt
          </h4>
          <div className="text-gray-700 text-sm">{req.attempt ?? "<unknown>"}</div>
        </div>
        <div className="mt-6">
          <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
            Originally Published
          </h4>
          <div className="text-gray-700 text-sm">
            {req.published ? DateTime.fromMillis(req.published).toString() : "<unknown>"}
          </div>
        </div>
      </div>
      <div className="mt-6">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Message
        </h4>
        {req.request_payload ? (
          <CodeBox>
            <PayloadViewer payload={req.request_payload} />
          </CodeBox>
        ) : (
          <div className="text-gray-700 text-sm">No message data.</div>
        )}
      </div>
      {req.err !== null ? (
        <div className="mt-4">
          <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
            Error{" "}
            <button className="text-gray-600 ml-1" onClick={() => onStackTrace(req.err_stack!)}>
              {icons.stackTrace("m-1 h-4 w-auto")}
            </button>
          </h4>
          <CodeBox error>{decodeBase64(req.err)}</CodeBox>
        </div>
      ) : undefined}
    </>
  ) : (
    <>
      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Request
        </h4>
        {isRaw ? (
          <RawRequestDetail req={req} />
        ) : req.request_payload ? (
          <CodeBox>
            <RequestURL method={req.http_method} path={req.path} />
            <PayloadViewer payload={req.request_payload} />
          </CodeBox>
        ) : (
          <div className="text-gray-700 text-sm">No request data.</div>
        )}
      </div>
      {req.err !== null ? (
        <div className="mt-4">
          <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
            Error{" "}
            <button className="text-gray-600 ml-1" onClick={() => onStackTrace(req.err_stack!)}>
              {icons.stackTrace("m-1 h-4 w-auto")}
            </button>
          </h4>
          <CodeBox error>{decodeBase64(req.err)}</CodeBox>
        </div>
      ) : (
        <div className="mt-4">
          <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
            Response
          </h4>
          {isRaw ? (
            <RawResponseDetail req={req} />
          ) : req.response_payload ? (
            <CodeBox>
              <PayloadViewer payload={req.response_payload} />
            </CodeBox>
          ) : (
            <div className="text-gray-700 text-sm">No response data.</div>
          )}
        </div>
      )}
    </>
  );
};

const RawRequestDetail: FC<{ req: Request }> = ({ req }) => {
  const [headersExpanded, setHeadersExpanded] = useState(false);
  return (
    <CodeBox>
      <div className="ml-1 text-white text-opacity-75">
        {req.http_method} {req.path}
      </div>

      {req.raw_req_headers.length > 0 && (
        <div className="ml-1 text-white text-opacity-75">
          <button
            className="flex items-center text-white text-opacity-75 hover:text-opacity-100"
            onClick={() => setHeadersExpanded((val) => !val)}
          >
            {(headersExpanded ? icons.minus : icons.plus)("border h-4 w-auto mr-2")}
            {req.raw_req_headers.length} Request Headers
          </button>
          {headersExpanded && (
            <div>
              {req.raw_req_headers.map((kv, i) => (
                <div key={i}>
                  {kv.key}: {kv.value}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {req.request_payload && (
        <div className="mt-2">
          <PayloadViewer payload={req.request_payload} />
        </div>
      )}
    </CodeBox>
  );
};

const RawResponseDetail: FC<{ req: Request }> = ({ req }) => {
  const [headersExpanded, setHeadersExpanded] = useState(false);
  return (
    <CodeBox>
      {req.raw_resp_headers.length > 0 && (
        <div className="ml-1 text-white text-opacity-75">
          <button
            className="flex items-center text-white text-opacity-75 hover:text-opacity-100"
            onClick={() => setHeadersExpanded((val) => !val)}
          >
            {(headersExpanded ? icons.minus : icons.plus)("border h-4 w-auto mr-2")}
            {req.raw_resp_headers.length} Response headers
          </button>
          {headersExpanded && (
            <div>
              {req.raw_resp_headers.map((kv, i) => (
                <div key={i}>
                  {kv.key}: {kv.value}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {req.response_payload && (
        <div className="mt-2">
          <PayloadViewer payload={req.response_payload} />
        </div>
      )}
    </CodeBox>
  );
};

const RequestURL: FC<{ method: string; path: string }> = (props) => {
  return (
    <p className="mb-2 block font-mono text-xs text-white">
      {props.method} <span className="text-white text-opacity-70">{props.path}</span>
    </p>
  );
};

const PayloadViewer: FC<{
  payload: Base64EncodedBytes | string;
  hideRoot?: boolean;
  shouldExpandNode?: CircularPropsPassedThroughJSONTree["shouldExpandNode"];
}> = ({ payload, hideRoot, shouldExpandNode }) => {
  let decoded = "";
  try {
    decoded = decodeBase64(payload);
  } catch (e) {
    decoded = payload;
  }
  let jsonObj: any = undefined;
  try {
    jsonObj = JSON.parse(decoded);
  } catch (err) {
    /* do nothing */
  }

  const theme: any = {
    scheme: "monokai",
    base00: "#111111",
    base01: "#383830",
    base02: "#49483e",
    base03: "#75715e",
    base04: "#a59f85",
    base05: "#f8f8f2",
    base06: "#f5f4f1",
    base07: "#f9f8f5",
    base08: "#f92672",
    base09: "#fd971f",
    base0A: "#f4bf75",
    base0B: "#a6e22e",
    base0C: "#a1efe4",
    base0D: "#66d9ef",
    base0E: "#ae81ff",
    base0F: "#cc6633",
  };

  return jsonObj !== undefined ? (
    <div className="json-tree whitespace-normal [&_svg]:inline-block [&_.data-key]:align-top">
      <JSONTree
        hideRoot={!!hideRoot}
        data={jsonObj}
        shouldExpandNode={shouldExpandNode ? shouldExpandNode : () => true}
        valueRenderer={(raw, value) => {
          if (typeof value === "string") {
            return <StringValueRenderer str={value} collapseStringsAfterLength={100} />;
          }
          return raw;
        }}
        labelRenderer={(keyPath) => {
          if (keyPath.length === 1 && keyPath[0] === "root") return "";
          return <span>{keyPath[0]}:</span>;
        }}
        getItemString={(type, data, itemType) => (
          <ItemStringWrapper data={data}>{itemType}</ItemStringWrapper>
        )}
        invertTheme={false}
        theme={{
          extend: theme,
          nestedNode: ({ style }, keyPath, nodeType, expanded) => ({
            className: "nested-node",
            style: {
              ...style,
              paddingLeft: "2px",
            },
          }),
        }}
      />
    </div>
  ) : (
    <CM
      cfg={{
        value: decoded,
        readOnly: true,
        theme: "encore",
        mode: { name: "javascript", json: true },
      }}
      noShadow={true}
    />
  );
};

const ItemStringWrapper: FC<PropsWithChildren & { data: any }> = (props) => {
  const [isCopied, setIsCopied] = useState(false);
  const className = "h-4 w-4 copy-icon text-lightgray cursor-pointer";
  const onClick = (e: any) => {
    e.stopPropagation();
    copyToClipboard(JSON.stringify(props.data));
    setIsCopied(true);
    setTimeout(() => {
      setIsCopied(false);
    }, 3000);
  };
  return (
    <span>
      {props.children}
      {isCopied ? (
        <ClipboardDocumentCheckIcon onClick={onClick} className={className} />
      ) : (
        <ClipboardDocumentIcon onClick={onClick} className={className} />
      )}
    </span>
  );
};

const StringValueRenderer: FC<{ str: string; collapseStringsAfterLength: number }> = (props) => {
  const [isExpanded, setIsExpanded] = useState(false);
  if (isExpanded || props.str.length < props.collapseStringsAfterLength) {
    return <span>"{props.str}"</span>;
  }
  return (
    <span className="cursor-pointer" onClick={() => setIsExpanded(true)}>
      "{props.str.slice(0, props.collapseStringsAfterLength)}..."
    </span>
  );
};

type gdata = {
  goid: number;
  start: number;
  end: number | undefined;
  events: Event[];
};

const EventMap: FunctionComponent<{
  req: Request;
  trace: Trace;
  onStackTrace: (s: Stack) => void;
}> = (props) => {
  const req = props.req;

  // Compute the list of interesting goroutines
  const gmap: { [key: number]: gdata } = {
    [req.goid]: {
      goid: req.goid,
      start: req.start_time,
      end: req.end_time,
      events: [],
    },
  };
  const gnums: number[] = [req.goid];

  for (const ev of req.events) {
    if (ev.type === "Goroutine") {
      gmap[ev.goid] = {
        goid: ev.goid,
        start: ev.start_time,
        end: ev.end_time,
        events: [],
      };
      gnums.push(ev.goid);
    } else if (ev.type === "DBTransaction") {
      let g = gmap[ev.goid];
      g.events = g.events.concat(ev.queries);
    } else {
      gmap[ev.goid].events.push(ev);
    }
  }

  const lines = gnums.map((n) => gmap[n]).filter((g) => g.events.length > 0 || g.goid === req.goid);
  return (
    <div>
      <div className="body-xs text-gray-400 mb-1 flex items-center">
        {icons.chip("h-4 w-auto")}
        <span className="text-gray-800 mx-1 font-bold">{lines.length}</span>
        Goroutine{lines.length !== 1 ? "s" : ""}
      </div>
      <div className="bg-white">
        {lines.map((g, i) => (
          <div key={g.goid} className={i > 0 ? "mt-0.5" : ""}>
            <GoroutineDetail
              key={g.goid}
              g={g}
              req={req}
              trace={props.trace}
              onStackTrace={props.onStackTrace}
            />
          </div>
        ))}
      </div>
    </div>
  );
};

const GoroutineDetail: FunctionComponent<{
  g: gdata;
  req: Request;
  trace: Trace;
  onStackTrace: (s: Stack) => void;
}> = (props) => {
  const req = props.req;
  const reqDur = req.end_time! - req.start_time;
  const start = Math.round(((props.g.start - req.start_time) / reqDur) * 100);
  const end = Math.round(((props.g.end! - req.start_time) / reqDur) * 100);
  const g = props.g;
  const gdur = g.end! - g.start;
  const lineHeight = 22;

  const tooltipRef = useRef<HTMLDivElement>(null);
  const goroutineEl = useRef<HTMLDivElement>(null);
  const [hoverObj, setHoverObj] = useState<Request | Event | null>(null);
  const [barOver, setBarOver] = useState(false);
  const [tooltipOver, setTooltipOver] = useState(false);

  const setHover = (ev: React.MouseEvent, obj: Request | Event | null) => {
    if (obj === null) {
      setBarOver(false);
      return;
    }

    const el = tooltipRef.current;
    const gel = goroutineEl.current;
    if (!el || !gel) {
      return;
    }

    setBarOver(true);
    setHoverObj(obj);
    const spanEl = ev.target as HTMLElement;
    const offset = spanEl.getBoundingClientRect();

    el.style.top = `calc(${offset.top}px - 40px)`;
    el.style.transform = `translateX(calc(-100% + ${gel.offsetLeft}px + ${spanEl.offsetLeft}px))`;
  };

  const barEvents: (DBQuery | RPCCall | HTTPCall | PubSubPublish | CacheOp)[] = g.events.filter(
    (e) =>
      e.type === "DBQuery" ||
      e.type === "RPCCall" ||
      e.type === "HTTPCall" ||
      e.type === "PubSubPublish" ||
      e.type === "CacheOp"
  ) as any;

  return (
    <>
      <div className="relative" style={{ height: lineHeight + "px" }}>
        <div
          ref={goroutineEl}
          className={`absolute`}
          style={{
            height: lineHeight + "px",
            left: start + "%",
            right: 100 - end + "%",
            minWidth: "3px", // so it at least renders
          }}
        >
          <div className="absolute inset-0 flex items-center">
            <div className="h-px w-full bg-lightgray" />
          </div>
          {barEvents.map((ev, i) => {
            const start = Math.round(((ev.start_time - g.start) / gdur) * 100);
            const end = Math.round(((ev.end_time! - g.start) / gdur) * 100);
            const clsid = `ev-${req.id}-${g.goid}-${i}`;

            if (ev.type === "DBQuery") {
              const [color, highlightColor] = idxColor(i);
              return (
                <div
                  key={i}
                  data-testid={clsid}
                  className={`span bg-[var(--base-color)] hover:bg-[var(--hover-color)] absolute inset-y-0`}
                  onMouseEnter={(e) => setHover(e, ev)}
                  onMouseLeave={(e) => setHover(e, null)}
                  style={
                    {
                      "--base-color": color,
                      "--hover-color": highlightColor,
                      top: "2px",
                      bottom: "2px",
                      left: start + "%",
                      right: 100 - end + "%",
                      minWidth: "1px", // so it at least renders if start === stop
                    } as CSSProperties
                  }
                />
              );
            } else if (ev.type === "RPCCall") {
              const defLoc = props.trace.locations[ev.def_loc];
              let svcName = "unknown";
              if ("rpc_def" in defLoc) {
                svcName = defLoc.rpc_def.service_name;
              }
              const [color, highlightColor] = idxColor(i);
              return (
                <div
                  key={i}
                  className={`span bg-[var(--base-color)] hover:bg-[var(--hover-color)] absolute inset-y-0`}
                  onMouseEnter={(e) => setHover(e, ev)}
                  onMouseLeave={(e) => setHover(e, null)}
                  style={
                    {
                      "--base-color": color,
                      "--hover-color": highlightColor,
                      top: "2px",
                      bottom: "2px",
                      left: start + "%",
                      right: 100 - end + "%",
                      minWidth: "1px", // so it at least renders if start === stop
                    } as CSSProperties
                  }
                />
              );
            } else if (ev.type === "HTTPCall") {
              const [color, highlightColor] = idxColor(i);
              return (
                <div
                  key={i}
                  className={`span bg-[var(--base-color)] hover:bg-[var(--hover-color)] absolute inset-y-0`}
                  onMouseEnter={(e) => setHover(e, ev)}
                  onMouseLeave={(e) => setHover(e, null)}
                  style={
                    {
                      "--base-color": color,
                      "--hover-color": highlightColor,
                      top: "2px",
                      bottom: "2px",
                      left: start + "%",
                      right: 100 - end + "%",
                      minWidth: "1px", // so it at least renders if start === stop
                    } as CSSProperties
                  }
                />
              );
            } else if (ev.type === "PubSubPublish") {
              const [color, highlightColor] = idxColor(i);
              return (
                <div
                  key={i}
                  className={`span bg-[var(--base-color)] hover:bg-[var(--hover-color)] absolute inset-y-0`}
                  onMouseEnter={(e) => setHover(e, ev)}
                  onMouseLeave={(e) => setHover(e, null)}
                  style={
                    {
                      "--base-color": color,
                      "--hover-color": highlightColor,
                      top: "2px",
                      bottom: "2px",
                      left: start + "%",
                      right: 100 - end + "%",
                      minWidth: "1px", // so it at least renders if start === stop
                    } as CSSProperties
                  }
                />
              );
            } else if (ev.type === "CacheOp") {
              const [color, highlightColor] = idxColor(i);
              return (
                <div
                  key={i}
                  className={`span bg-[var(--base-color)] hover:bg-[var(--hover-color)] absolute inset-y-0`}
                  onMouseEnter={(e) => setHover(e, ev)}
                  onMouseLeave={(e) => setHover(e, null)}
                  style={
                    {
                      "--base-color": color,
                      "--hover-color": highlightColor,
                      top: "2px",
                      bottom: "2px",
                      left: start + "%",
                      right: 100 - end + "%",
                      minWidth: "1px", // so it at least renders if start === stop
                    } as CSSProperties
                  }
                />
              );
            }
          })}
        </div>
      </div>
      <div
        data-testid="trace-tooltip"
        ref={tooltipRef}
        className="absolute z-40 w-full max-w-md pr-2"
        style={{
          width: "500px",
          paddingRight: "10px" /* extra padding to make it easier to hover into the tooltip */,
        }}
        onMouseEnter={() => setTooltipOver(true)}
        onMouseLeave={() => setTooltipOver(false)}
      >
        {(barOver || tooltipOver) && (
          <div className="w-full overflow-auto border-2 border-black bg-white p-3">
            {hoverObj &&
              "type" in hoverObj &&
              (hoverObj.type === "DBQuery" ? (
                <DBQueryTooltip
                  q={hoverObj}
                  trace={props.trace}
                  onStackTrace={props.onStackTrace}
                />
              ) : hoverObj.type === "RPCCall" ? (
                <RPCCallTooltip
                  call={hoverObj as RPCCall}
                  req={req}
                  trace={props.trace}
                  onStackTrace={props.onStackTrace}
                />
              ) : hoverObj.type === "HTTPCall" ? (
                <HTTPCallTooltip call={hoverObj as HTTPCall} req={req} trace={props.trace} />
              ) : hoverObj.type === "PubSubPublish" ? (
                <PubsubPublishTooltip
                  publish={hoverObj}
                  trace={props.trace}
                  onStackTrace={props.onStackTrace}
                />
              ) : hoverObj.type === "CacheOp" ? (
                <CacheOpTooltip
                  op={hoverObj}
                  trace={props.trace}
                  onStackTrace={props.onStackTrace}
                />
              ) : null)}
          </div>
        )}
      </div>
    </>
  );
};

const PubsubPublishTooltip: FunctionComponent<{
  publish: PubSubPublish;
  trace: Trace;
  onStackTrace: (s: Stack) => void;
}> = (props) => {
  const publish = props.publish;
  return (
    <div>
      <h3 className="flex items-center text-lg font-bold text-black">
        {icons.arrowsExpand("h-8 w-auto text-gray-400 mr-2")}
        Publish: {publish.topic}
        <div className="text-gray-500 ml-auto flex items-center text-sm font-normal">
          {publish.end_time ? latencyStr(publish.end_time - publish.start_time) : "Unknown"}
          <button
            className="-mr-1 focus:outline-none"
            onClick={() => props.onStackTrace(publish.stack)}
          >
            {icons.stackTrace("m-1 h-4 w-auto")}
          </button>
        </div>
      </h3>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Message ID
        </h4>
        <div className="text-gray-700 text-sm">{publish.message_id ?? <i>Not Sent</i>}</div>
      </div>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Message
        </h4>
        {renderData([publish.message], "max-h-96")}
      </div>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Error
        </h4>
        {publish.err !== null ? (
          <CodeBox error>{decodeBase64(publish.err)}</CodeBox>
        ) : (
          <div className="text-gray-700 text-sm">Completed successfully.</div>
        )}
      </div>
    </div>
  );
};

const CacheOpTooltip: FunctionComponent<{
  op: CacheOp;
  trace: Trace;
  onStackTrace: (s: Stack) => void;
}> = (props) => {
  const op = props.op;
  const defLoc = props.trace.locations[op.def_loc];
  let keyspaceName: string | undefined;
  if (defLoc && "cache_keyspace" in defLoc) {
    keyspaceName = defLoc.cache_keyspace.var_name;
  }

  return (
    <div>
      <h3 className="flex items-center text-lg font-bold text-black">
        {(op.write ? icons.archiveBoxArrowDown : icons.archiveBoxArrowUp)(
          "h-8 w-auto text-gray-400 mr-2"
        )}
        Cache {op.write ? "Write" : "Read"}
        <div className="text-gray-500 ml-auto flex items-center text-sm font-normal">
          {op.end_time ? latencyStr(op.end_time - op.start_time) : "Unknown"}
          {op.stack.frames.length > 0 && (
            <button
              className="-mr-1 focus:outline-none"
              onClick={() => props.onStackTrace(op.stack)}
            >
              {icons.stackTrace("m-1 h-4 w-auto")}
            </button>
          )}
        </div>
      </h3>

      {keyspaceName && (
        <div className="mt-4">
          <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
            Keyspace
          </h4>
          <div className="text-gray-700 text-sm">{keyspaceName}</div>
        </div>
      )}

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Operation
        </h4>
        <div className="text-gray-700 text-sm">{op.operation}</div>
      </div>

      {op.keys.length > 0 && (
        <div className="mt-4">
          <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
            {op.keys.length !== 1 ? "Keys" : "Key"}
          </h4>
          <CodeBox>{op.keys.join("\n")}</CodeBox>
        </div>
      )}

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Result
        </h4>
        {op.err ? (
          <CodeBox error>{decodeBase64(op.err)}</CodeBox>
        ) : (
          <div className="text-gray-700 text-sm">
            {op.result === CacheResult.NoSuchKey
              ? "Key not found"
              : op.result === CacheResult.Conflict
              ? "Precondition failed"
              : op.result === CacheResult.Ok
              ? "Completed successfully"
              : "Unknown"}
          </div>
        )}
      </div>
    </div>
  );
};

const DBQueryTooltip: FunctionComponent<{
  q: DBQuery;
  trace: Trace;
  onStackTrace: (s: Stack) => void;
}> = (props) => {
  const q = props.q;
  return (
    <div>
      <h3 className="flex items-center text-lg font-bold text-black">
        {icons.database("h-8 w-auto text-gray-400 mr-2")}
        DB Query
        <div className="text-gray-500 ml-auto flex items-center text-sm font-normal">
          {q.end_time ? latencyStr(q.end_time - q.start_time) : "Unknown"}
          <button className="-mr-1 focus:outline-none" onClick={() => props.onStackTrace(q.stack)}>
            {icons.stackTrace("m-1 h-4 w-auto")}
          </button>
        </div>
      </h3>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Query
        </h4>
        {renderData([q.query], "max-h-96", "sql")}
      </div>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Error
        </h4>
        {q.err !== null ? (
          <CodeBox error>{decodeBase64(q.err)}</CodeBox>
        ) : (
          <div className="text-gray-700 text-sm">Completed successfully.</div>
        )}
      </div>
    </div>
  );
};

const RPCCallTooltip: FunctionComponent<{
  call: RPCCall;
  req: Request;
  trace: Trace;
  onStackTrace: (s: Stack) => void;
}> = (props) => {
  const c = props.call;
  const target = props.req.children.find((r) => r.id === c.req_id);
  const defLoc = props.trace.locations[c.def_loc];
  let endpoint: string | null = null;
  if ("rpc_def" in defLoc) {
    endpoint = `${defLoc.rpc_def.service_name}.${defLoc.rpc_def.rpc_name}`;
  }

  return (
    <div>
      <h3 className="flex items-center text-lg font-bold text-black">
        {icons.logout("h-8 w-auto text-gray-400 mr-2")}
        API Call
        {endpoint !== null ? (
          <span>: {endpoint}</span>
        ) : (
          <span className="text-gray-500 text-sm italic">Unknown Endpoint</span>
        )}
        <div className="text-gray-500 ml-auto flex items-center text-sm font-normal">
          {c.end_time ? latencyStr(c.end_time - c.start_time) : "Unknown"}
          <button className="-mr-1 focus:outline-none" onClick={() => props.onStackTrace(c.stack)}>
            {icons.stackTrace("m-1 h-4 w-auto")}
          </button>
        </div>
      </h3>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Request
        </h4>
        {target !== undefined ? (
          renderRequestPayload(getRequestInfo(props.trace, target))
        ) : (
          <div className="text-sm">Not captured.</div>
        )}
      </div>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Response
        </h4>
        {target !== undefined ? (
          target.outputs.length > 0 ? (
            renderData(target.outputs, "max-h-96")
          ) : target.response_payload ? (
            renderPayload(target.response_payload)
          ) : (
            <div className="text-gray-700 text-sm">No response data.</div>
          )
        ) : (
          <div className="text-gray-700 text-sm">Not captured.</div>
        )}
      </div>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Error
        </h4>
        {c.err !== null ? (
          <pre className="overflow-auto rounded border bg-black p-2 text-sm text-white">
            {decodeBase64(c.err)}
          </pre>
        ) : (
          <div className="text-gray-700 text-sm">Completed successfully.</div>
        )}
      </div>
    </div>
  );
};

const HTTPCallTooltip: FunctionComponent<{
  call: HTTPCall;
  req: Request;
  trace: Trace;
}> = ({ call, req, trace }) => {
  const m = call.metrics;
  return (
    <div>
      <h3 className="text-gray-800 flex items-center text-lg font-bold">
        {icons.logout("h-8 w-auto text-gray-400 mr-2")}
        HTTP {call.method} {call.host}
        {call.path}
        <div className="text-gray-500 ml-auto flex items-center text-sm font-normal">
          {call.end_time ? latencyStr(call.end_time - call.start_time) : "Unknown"}
        </div>
      </h3>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          URL
        </h4>
        <pre className="border-gray-200 bg-gray-100 text-gray-800 overflow-auto rounded border p-2 text-sm">
          {call.url}
        </pre>
      </div>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Response
        </h4>
        {call.end_time !== -1 ? (
          <div className="text-gray-700 text-sm">HTTP {call.status_code}</div>
        ) : (
          <div className="text-gray-700 text-sm">No response recorded.</div>
        )}
      </div>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Error
        </h4>
        {call.err !== null ? (
          <pre className="overflow-auto rounded border bg-black p-2 text-sm text-white">
            {decodeBase64(call.err)}
          </pre>
        ) : (
          <div className="text-gray-700 text-sm">Completed successfully.</div>
        )}
      </div>

      <div className="mt-4">
        <h4 className="text-gray-300 mb-2 font-sans text-xs font-semibold uppercase leading-3 tracking-wider">
          Timeline
        </h4>
        <div className="text-gray-600 inline-grid grid-cols-2 text-xs">
          {m.conn_reused ? (
            <>
              <span>Reused Connection:</span> <span className="text-right">Yes</span>
            </>
          ) : (
            <>
              {m.dns_done && (
                <>
                  <span>DNS Lookup:</span>{" "}
                  <span className="text-right">{latencyStr(m.dns_done - call.start_time)}</span>
                </>
              )}
              {m.tls_handshake_done && (
                <>
                  <span>TLS Handshake:</span>{" "}
                  <span className="text-right">
                    {latencyStr(m.tls_handshake_done - (m.dns_done ?? call.start_time))}
                  </span>
                </>
              )}
            </>
          )}
          {m.wrote_request && (
            <>
              <span>Wrote Request:</span>{" "}
              <span className="text-right">
                {latencyStr(
                  m.wrote_request - (m.tls_handshake_done ?? m.got_conn ?? call.start_time)
                )}
              </span>
            </>
          )}
          {m.first_response && (
            <>
              <span>Response Start:</span>{" "}
              <span className="text-right">
                {latencyStr(m.first_response - (m.wrote_headers ?? m.got_conn ?? call.start_time))}
              </span>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

const renderData = (
  data: Base64EncodedBytes[],
  className: string = "",
  mode: string | ModeSpec<ModeSpecOptions> = {
    name: "javascript",
    json: true,
  }
) => {
  const raw = decodeBase64(data[0]);
  let pretty = raw;
  try {
    const json = JSON.parse(raw);
    pretty = JSON.stringify(json, undefined, "  ");
  } catch (err) {
    /* do nothing */
  }
  return (
    <CodeBox className={className}>
      <CM
        key={pretty}
        cfg={{
          value: pretty,
          readOnly: true,
          theme: "encore",
          mode: mode,
        }}
        noShadow={true}
      />
    </CodeBox>
  );
};

interface RequestInfo {
  payload: string | undefined;
  pathParams: [string, string][]; // [key, value][]
}

function getRequestInfo(tr: Trace, req: Request): RequestInfo {
  const svc = tr.meta.svcs.find((s) => s.name === req.svc_name);
  const rpc = svc?.rpcs.find((r) => r.name === req.rpc_name);
  const paramSpec = rpc?.path.segments.filter((s) => s.type !== "LITERAL");

  let payload: string | undefined;
  let pathParams: [string, string][];
  if (req.inputs.length > 0) {
    // Legacy format
    const raw = req.inputs.map((e) => decodeBase64(e));
    payload = raw.length > (paramSpec?.length ?? 0) ? raw[raw.length - 1] : undefined;
    pathParams = paramSpec?.map((seg, i) => [seg.value, raw[i]]) ?? [];
  } else {
    payload = decodeBase64(req.request_payload ?? "");
    pathParams = paramSpec?.map((seg, i) => [seg.value, req.path_params[i]]) ?? [];
  }

  if (payload !== undefined) {
    try {
      const json = JSON.parse(payload);
      payload = JSON.stringify(json, undefined, "  ");
    } catch (err) {
      /* do nothing */
    }
  }

  return { payload, pathParams };
}

const renderRequestPayload = (info: RequestInfo, ctx: "request" | "pubsub" = "request") => {
  if (info.pathParams.length === 0 && !info.payload) {
    return (
      <div className="text-sm">{ctx === "pubsub" ? "No message data" : "No request data."}</div>
    );
  }

  const showPath = ctx === "request";

  return (
    <CodeBox>
      {showPath &&
        info.pathParams.map((p, i) => (
          <div key={i}>
            <span className="text-gray-400">{p[0]}:</span> {p[1]}
          </div>
        ))}
      {info.payload && (
        <div>
          {showPath && info.pathParams.length > 0 && (
            <span className="text-gray-400">
              <br />
            </span>
          )}
          <CM
            cfg={{
              value: info.payload,
              readOnly: true,
              theme: "encore",
              mode: { name: "javascript", json: true },
            }}
            noShadow={true}
          />
        </div>
      )}
    </CodeBox>
  );
};

const renderPayload = (data: Base64EncodedBytes) => {
  data = decodeBase64(data);
  try {
    const json = JSON.parse(data);
    data = JSON.stringify(json, undefined, "  ");
  } catch (err) {
    /* do nothing */
  }

  return (
    <CodeBox>
      <CM
        key={data} /* needed to re-render on request changes */
        cfg={{
          value: data,
          readOnly: true,
          theme: "encore",
          mode: { name: "javascript", json: true },
        }}
        noShadow={true}
      />
    </CodeBox>
  );
};

const renderLogWithJSON = (tr: Trace, log: LogMessage, key: any, expandFields: boolean) => {
  let dt = timeToDate(tr.date)!;
  const ms = (log.time - tr.start_time) / 1000;
  dt = dt.plus(Duration.fromMillis(ms));

  const payload = useMemo<string>(() => {
    const obj: Record<string, any> = {};
    log.fields.forEach((logField, index) => (obj[logField.key] = logField.value));
    return JSON.stringify(obj);
  }, [log.fields]);

  return (
    <div key={key} className="mb-5 last:mb-0">
      <span className="text-lightgray">{dt.toFormat("HH:mm:ss.SSS")} </span>
      {log.level === "TRACE" ? (
        <span className="text-lightgray">TRC </span>
      ) : log.level === "DEBUG" ? (
        <span className="text-lightgray">DBG </span>
      ) : log.level === "INFO" ? (
        <span className="text-codeblue">INF </span>
      ) : log.level === "WARN" ? (
        <span className="text-codeorange">WRN </span>
      ) : (
        <span className="text-red">ERR </span>
      )}
      {log.msg}
      <PayloadViewer
        hideRoot
        payload={payload}
        shouldExpandNode={(keyPath, data, level) => {
          if (expandFields) return true;
          // collapse first level objects by default to get an overview of the payload
          if (level === 1 && typeof data === "object") return false;
          return true;
        }}
      />
    </div>
  );
};

function findCall(tr: Trace, id: string): RPCCall | undefined {
  const queue: Request[] = [];
  if (tr.root !== null) {
    queue.push(tr.root);
  }

  while (queue.length > 0) {
    const req = queue.shift()!;
    for (const e of req.events) {
      if (e.type === "RPCCall" && e.req_id === id) {
        return e;
      }
    }
    queue.push(...req.children);
  }
  return undefined;
}

const CodeBox: FC<PropsWithChildren<{ className?: string; error?: boolean }>> = (props) => {
  return (
    <pre
      className={`response-docs overflow-auto rounded border bg-black p-2 text-sm ${
        props.error ? "text-red" : "text-white"
      } ${props.className ?? ""}`}
    >
      {props.children}
    </pre>
  );
};
