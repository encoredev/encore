import { Listbox, Menu, Transition } from "@headlessui/react";
import CodeMirror, { EditorConfiguration } from "codemirror";
import HJSON from "hjson";
import React, { FC, useEffect, useImperativeHandle, useRef, useState } from "react";
import Button from "~c/Button";
import { icons } from "~c/icons";
import Input from "~c/Input";
import { decodeBase64, encodeBase64 } from "~lib/base64";
import JSONRPCConn from "~lib/client/jsonrpc";
import { copyToClipboard } from "~lib/clipboard";
import { APIEncoding, APIMeta, ParameterEncoding, PathSegment, RPC, Service } from "./api";
import CM from "./cm/CM";
import { Builtin, NamedType } from "./schema";
import { JSONDialect } from "~c/api/SchemaView";

interface Props {
  conn: JSONRPCConn;
  appID: string;
  md: APIMeta;
  apiEncoding: APIEncoding;
  svc: Service;
  rpc: RPC;
  addr?: string;
}

export const cfg: EditorConfiguration = {
  theme: "encore",
  mode: "json",
  lineNumbers: false,
  lineWrapping: false,
  indentWithTabs: true,
  indentUnit: 4,
  tabSize: 4,
  autoCloseBrackets: true,
  matchBrackets: true,
  styleActiveLine: false,
};

const RPCCaller: FC<Props> = ({ md, apiEncoding, svc, rpc, conn, appID, addr }) => {
  const payloadCM = useRef<CM>(null);
  const authCM = useRef<CM>(null);
  const pathRef = useRef<{
    getPath: () => string | undefined;
    getMethod: () => string;
  }>(null);
  const docs = useRef(new Map<RPC, CodeMirror.Doc>());
  const authDoc = useRef<CodeMirror.Doc>(
    new CodeMirror.Doc("", {
      name: "javascript",
      json: true,
    })
  );
  const authGeneratedJS = useRef("");
  const [authToken, setAuthToken] = useState("");
  const hasPathParams = rpc.path?.segments.findIndex((s) => s.type !== "LITERAL") !== -1 ?? false;

  const [loading, setLoading] = useState(false);
  const [respErr, setRespErr] = useState<string | undefined>(undefined);
  const [response, setResponse] = useState<string | undefined>(undefined);
  const [method, setMethod] = useState<string>(rpc.http_methods[0]);

  const serializeRequest = (): [string, string, string] => {
    const path = pathRef.current?.getPath() ?? `/${svc.name}.${rpc.name}`;
    let body = "";

    if (rpc.request_schema) {
      const doc = docs.current.get(rpc);
      if (doc === undefined) {
        return ["", "", ""];
      }
      body = doc.getValue();
    }

    return [path, body, authDoc.current.getValue()];
  };

  const makeRequest = async () => {
    const [path, reqBody, authBody] = serializeRequest();
    if (path === "") {
      return;
    }

    try {
      setLoading(true);
      setResponse(undefined);
      setRespErr(undefined);
      const resp = (await conn.request("api-call", {
        appID,
        service: svc.name,
        endpoint: rpc.name,
        method,
        path,
        auth_payload: encodeBase64(authBody),
        auth_token: authToken,
        payload: encodeBase64(reqBody),
      })) as any;
      let respBody = "";
      if (resp.body.length > 0) {
        respBody = decodeBase64(resp.body);
      }

      if (resp.status_code !== 200) {
        setRespErr(`HTTP ${resp.status}: ${respBody}`);
      } else if (rpc.response_schema) {
        setResponse(respBody);
      } else {
        setResponse("Request completed successfully.");
      }
    } catch (err) {
      setRespErr(`Internal Error: ${err}`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (method !== rpc.http_methods[0]) {
      setMethod(rpc.http_methods[0]);
    }
  }, [rpc]);

  function namedTypeToHJSON(named: NamedType): string {
    const render = new JSONDialect(md);
    render.method = method;
    render.asResponse = false;
    render.typeArgumentStack.push(named.type_arguments);
    const structType = md.decls[named.id].type.struct!;
    const [queryString, headers, js] = render.structBits(structType, false, true);

    const bits: string[] = ["{\n"];
    let previousSection = false;
    if (headers) {
      bits.push("    // HTTP headers", headers);
      previousSection = true;
    }
    if (queryString) {
      if (previousSection) {
        bits.push(",\n\n");
      }

      bits.push("    // Query string", queryString);
      previousSection = true;
    }
    if (js) {
      if (previousSection) {
        bits.push(",\n\n");
      }

      bits.push("    // HTTP body", js);
    }
    bits.push("\n}");

    return bits.join("");
  }

  useEffect(() => {
    if (rpc.request_schema) {
      const doc = new CodeMirror.Doc(namedTypeToHJSON(rpc.request_schema.named!), {
        name: "javascript",
        json: true,
      });
      docs.current.set(rpc, doc);
      payloadCM.current?.open(doc);
    }

    setResponse(undefined);
    setRespErr(undefined);
  }, [rpc, method]);

  useEffect(() => {
    if (md.auth_handler?.params?.named) {
      const generated = namedTypeToHJSON(md.auth_handler.params.named);

      if (authGeneratedJS.current !== generated) {
        authDoc.current.setValue("// Authentication Data\n" + generated);
        authCM.current?.open(authDoc.current);
        authGeneratedJS.current = generated;
      }
    }
  });

  const copyCurl = () => {
    let [path, reqBody, authBody] = serializeRequest();
    copyAsCurlToClipboard({
      serializeRequest: {
        path,
        reqBody,
        authBody,
      },
      addr,
      method,
      apiEncoding,
      rpc,
    });
  };

  return (
    <div>
      <h4 className="text-bold text-base">Request</h4>
      <div
        className={`mt-1 flex flex-col space-y-2 text-xs ${
          rpc.request_schema || hasPathParams || md.auth_handler ? "block" : "hidden"
        }`}
      >
        <div className="block bg-black p-1">
          <RPCPathEditor ref={pathRef} rpc={rpc} method={method} setMethod={setMethod} />
        </div>
        <div className={`${rpc.request_schema ? "block bg-black p-1" : " hidden"}`}>
          <CM ref={payloadCM} cfg={cfg} />
        </div>
        {md.auth_handler && md.auth_handler.params?.named !== undefined && (
          <div className="bg-black p-1">
            <CM ref={authCM} cfg={cfg} />
          </div>
        )}
      </div>
      <div
        className={`mt-1 text-xs ${
          rpc.request_schema || (md.auth_handler && md.auth_handler.params?.named)
            ? "hidden"
            : "block"
        }`}
      >
        This API takes no request data.
      </div>
      <div className="mt-1 flex items-center">
        {md.auth_handler && md.auth_handler.params?.builtin === Builtin.STRING && (
          <div className="shadow-sm relative mr-1 min-w-0 flex-1 rounded-md">
            <Input
              id=""
              className="w-full"
              label="Auth Token"
              required={rpc.access_type === "AUTH"}
              value={authToken}
              onChange={setAuthToken}
              noInputWrapper
            />
          </div>
        )}
        <APICallButton send={makeRequest} copyCurl={copyCurl} />
      </div>

      <h4 className="text-bold mt-4 mb-1 flex items-center text-base">
        Response {loading && icons.loading("ml-1 h-5 w-5", "#111111", "transparent", 4)}
      </h4>
      {response ? (
        <pre className="shadow-inner response-docs overflow-x-auto rounded bg-black p-2 text-xs text-white">
          <CM
            key={response}
            cfg={{
              value: response,
              readOnly: true,
              theme: "encore",
              mode: { name: "javascript", json: true },
            }}
            noShadow={true}
          />
        </pre>
      ) : respErr ? (
        <div className="overflow-x-auto bg-black p-2 font-mono text-xs text-red">{respErr}</div>
      ) : (
        <div className="text-gray-400 text-xs">Make a request to see the response.</div>
      )}
    </div>
  );
};

export default RPCCaller;

const APICallButton: FC<{ send: () => void; copyCurl: () => void }> = (props) => {
  return (
    <span className="shadow-sm relative z-0 ml-auto inline-flex flex-none rounded-md">
      <Button kind="primary" onClick={() => props.send()}>
        Call API
      </Button>
      <span className="relative z-10 ml-1 block">
        <Menu>
          {({ open }) => (
            <>
              <Menu.Button className="group relative h-full text-sm font-medium focus:z-10 focus:outline-none focus:ring-0">
                <div className="absolute inset-0 bg-gradient-to-r brandient-5" />
                <div className="relative inline-flex h-full items-center bg-black px-4 py-2 transition-transform duration-100 ease-in-out group-hover:-translate-x-1 group-hover:-translate-y-1">
                  <div className="text-white">{icons.chevronDown("h-4 w-4")}</div>
                  <span className="sr-only">Open options</span>
                </div>
              </Menu.Button>

              <Transition
                show={open}
                enter="transition ease-out duration-100"
                enterFrom="transform opacity-0 scale-95"
                enterTo="transform opacity-100 scale-100"
                leave="transition ease-in duration-75"
                leaveFrom="transform opacity-100 scale-100"
                leaveTo="transform opacity-0 scale-95"
              >
                <Menu.Items
                  static
                  className="border-gray-200 divide-gray-100 shadow-lg absolute right-0 mt-2 w-56 origin-top-right divide-y rounded-md border bg-white outline-none"
                >
                  <div className="py-1">
                    <Menu.Item>
                      {({ active }) => (
                        <button
                          className={`${
                            active ? "bg-gray-100 text-gray-900" : "text-gray-700"
                          } flex w-full justify-between px-4 py-2 text-left text-sm leading-5`}
                          onClick={() => props.copyCurl()}
                        >
                          Copy as curl
                        </button>
                      )}
                    </Menu.Item>
                  </div>
                </Menu.Items>
              </Transition>
            </>
          )}
        </Menu>
      </span>
    </span>
  );
};

export const pathEditorCfg: EditorConfiguration = {
  theme: "encore",
  mode: "json",
  lineNumbers: false,
  lineWrapping: false,
  indentWithTabs: true,
  indentUnit: 4,
  tabSize: 4,
  autoCloseBrackets: true,
  matchBrackets: true,
  styleActiveLine: false,
  extraKeys: {
    Tab: (cm: CodeMirror.Editor) => {
      const doc = cm.getDoc();
      const cur = doc.getCursor();
      if (!cur) {
        return;
      }
      const markers = (doc.getAllMarks() as CodeMirror.TextMarker<CodeMirror.MarkerRange>[])
        .filter((m) => !m.readOnly)
        .map((m) => m.find())
        .filter((m) => m !== undefined)
        .sort((a, b) => {
          return a!.from.ch - b!.from.ch;
        });

      for (let i = 0; i < markers.length; i++) {
        const m = markers[i];
        if (m!.from.ch <= cur.ch && m!.to.ch >= cur.ch) {
          if (i + 1 < markers.length) {
            const m2 = markers[i + 1];
            doc.setSelection(m2!.from, m2!.to);
          } else if (i > 0) {
            const m2 = markers[0];
            doc.setSelection(m2!.from, m2!.to);
          }
          return;
        }
      }
    },
    "Shift-Tab": (cm: CodeMirror.Editor) => {
      const doc = cm.getDoc();
      const cur = doc.getCursor();
      if (!cur) {
        return;
      }
      const markers = (doc.getAllMarks() as CodeMirror.TextMarker<CodeMirror.MarkerRange>[])
        .filter((m) => !m.readOnly)
        .map((m) => m.find())
        .filter((m) => m !== undefined)
        .sort((a, b) => {
          return a!.from.ch - b!.from.ch;
        });

      for (let i = 0; i < markers.length; i++) {
        const m = markers[i];
        if (m!.from.ch <= cur.ch && m!.to.ch >= cur.ch) {
          if (i - 1 >= 0) {
            const m2 = markers[i - 1];
            doc.setSelection(m2!.from, m2!.to);
          } else if (markers.length > 1) {
            const m2 = markers[markers.length - 1];
            doc.setSelection(m2!.from, m2!.to);
          }
          return;
        }
      }
    },
  },
};

function classNames(...classes: string[]) {
  return classes.filter(Boolean).join(" ");
}

const RPCPathEditor = React.forwardRef<
  { getPath: () => string | undefined; getMethod: () => string },
  { rpc: RPC; method: string; setMethod: (m: string) => void }
>(({ rpc, method, setMethod }, ref) => {
  interface DocState {
    rpc: RPC;
    doc: CodeMirror.Doc;
    markers: CodeMirror.TextMarker<CodeMirror.MarkerRange>[];
  }

  const pathCM = useRef<CM>(null);
  const docs = useRef(new Map<RPC, DocState>());
  const docMap = useRef(new Map<CodeMirror.Doc, DocState>());
  const timeoutHandle = useRef<{ id: any | null }>({ id: null });

  // Reset the method when the RPC changes
  useEffect(() => {
    setMethod(rpc.http_methods[0]);
  }, [rpc]);

  useEffect(() => {
    const segments: string[] = [];

    type rwSegment = {
      from: number;
      to: number;
      placeholder: string;
      seg: PathSegment;
    };
    const readWrites: rwSegment[] = [];
    let pos = 0;
    for (const s of rpc.path.segments) {
      segments.push("/");
      pos += 1;

      const placeholder = (s.type === "PARAM" ? ":" : s.type === "WILDCARD" ? "*" : "") + s.value;
      const ln = placeholder.length;
      segments.push(placeholder);
      if (s.type !== "LITERAL") {
        readWrites.push({ placeholder, seg: s, from: pos, to: pos + ln });
      }
      pos += ln;
    }

    const val = segments.join("");
    const doc = new CodeMirror.Doc(val);

    let prevEnd = 0;
    let i = 0;
    const markers: CodeMirror.TextMarker<CodeMirror.MarkerRange>[] = [];
    for (const rw of readWrites) {
      doc.markText(
        { ch: prevEnd, line: 0 },
        { ch: rw.from, line: 0 },
        {
          atomic: true,
          readOnly: true,
          clearWhenEmpty: false,
          clearOnEnter: false,
          className: "text-white text-opacity-70",
          selectLeft: i > 0,
          selectRight: true,
        }
      );
      const m = doc.markText(
        { ch: rw.from, line: 0 },
        { ch: rw.to, line: 0 },
        {
          className: "text-codeorange",
          clearWhenEmpty: false,
          clearOnEnter: false,
          inclusiveLeft: true,
          inclusiveRight: true,
          attributes: { placeholder: rw.placeholder, segmentType: rw.seg.type },
        }
      );
      markers.push(m);
      m.on("beforeCursorEnter", () => {
        const r = m.find();
        const sel = doc.getSelection();
        if (r) {
          const text = doc.getRange(r.from, r.to);
          if (text === m.attributes?.placeholder && sel !== text) {
            if (timeoutHandle.current.id) {
              clearTimeout(timeoutHandle.current.id);
            }
            timeoutHandle.current.id = setTimeout(() => {
              doc.setSelection(r.from, r.to);
            }, 50);
          }
        }
      });
      prevEnd = rw.to;
      i++;
    }

    doc.markText(
      { ch: prevEnd, line: 0 },
      { ch: val.length, line: 0 },
      {
        atomic: true,
        readOnly: true,
        clearWhenEmpty: false,
        clearOnEnter: false,
        className: "text-white text-opacity-70",
        selectLeft: prevEnd > 0,
        selectRight: false,
      }
    );

    CodeMirror.on(
      doc,
      "beforeChange",
      (doc: CodeMirror.Doc, change: CodeMirror.EditorChangeCancellable) => {
        if (change.text[0].indexOf("/") === -1) {
          return;
        }

        for (const m of markers) {
          const r = m.find();
          if (r && change.from.ch >= r.from.ch && change.from.ch <= r.to.ch) {
            if (m.attributes?.segmentType === "PARAM") {
              change.cancel();
            }
            return;
          }
        }
      }
    );

    const ds = { rpc, doc, markers: markers };
    docs.current.set(rpc, ds);
    docMap.current.set(doc, ds);
    pathCM.current?.open(ds.doc);
  }, [rpc, method]);

  useImperativeHandle(ref, () => {
    // noinspection JSUnusedGlobalSymbols
    return {
      getPath: () => pathCM.current?.cm?.getValue(),
      getMethod: () => method,
    };
  });

  return (
    <div className="flex items-center">
      {rpc.http_methods.length > 1 ? (
        <Listbox value={method} onChange={setMethod}>
          {({ open }) => (
            <div className="relative">
              <Listbox.Button className="hover:bg-green-200 relative block cursor-default rounded-sm py-0.5 pl-1 pr-5 text-left font-mono text-xs font-semibold text-codegreen focus:outline-none">
                <span className="block truncate">{method}</span>
                <span className="pointer-events-none absolute inset-y-0 right-0 flex items-center">
                  {icons.chevronDown("h-3 w-3 mr-1")}
                </span>
              </Listbox.Button>
              <Transition
                show={open}
                leave="transition ease-in duration-100"
                leaveFrom="opacity-100"
                leaveTo="opacity-0"
              >
                <Listbox.Options
                  static
                  className="absolute z-10 max-h-60 w-32 overflow-auto bg-white text-xs ring-1 ring-black ring-opacity-5 focus:outline-none"
                >
                  {rpc.http_methods.map((m) => (
                    <Listbox.Option
                      key={m}
                      className={({ active }) =>
                        classNames(
                          active ? "bg-black text-white" : "text-black",
                          "relative cursor-default select-none py-1 pl-3 pr-9"
                        )
                      }
                      value={m}
                    >
                      {({ selected, active }) => (
                        <>
                          <span
                            className={classNames(
                              selected ? "font-semibold" : "font-normal",
                              "block truncate"
                            )}
                          >
                            {m}
                          </span>

                          {selected ? (
                            <span
                              className={classNames(
                                active ? "text-white" : "text-green-600",
                                "absolute inset-y-0 right-0 flex items-center pr-4"
                              )}
                            >
                              {icons.check("h-5 w-5")}
                            </span>
                          ) : null}
                        </>
                      )}
                    </Listbox.Option>
                  ))}
                </Listbox.Options>
              </Transition>
            </div>
          )}
        </Listbox>
      ) : (
        <div className="px-1 font-mono text-xs text-white">{method}</div>
      )}
      <div className="flex-1">
        <CM ref={pathCM} cfg={pathEditorCfg} className="overflow-visible" />
      </div>
    </div>
  );
});

export const copyAsCurlToClipboard = (options: {
  serializeRequest: { path: any; reqBody: any; authBody: any };
  method: string;
  addr: string | undefined;
  apiEncoding: APIEncoding;
  rpc: RPC;
}) => {
  let { rpc, apiEncoding, method, addr } = options;
  let { path, reqBody, authBody } = options.serializeRequest;
  if (path === "") {
    return;
  }

  let headers: Record<string, any> = {};
  let queryString = "";

  function addQuery(name: string, value: any) {
    if (Array.isArray(value)) {
      return value.map((v) => {
        addQuery(name, v);
      });
    }

    if (queryString) {
      queryString += "&";
    } else {
      queryString = "?";
    }
    queryString += name + "=" + encodeURIComponent(value);
  }

  const newBody: Record<string, any> = {};

  function processStruct(encodedParams: ParameterEncoding[], payload: string) {
    try {
      const bodyFields: Record<string, any> = HJSON.parse(payload);
      if (typeof bodyFields !== "object") {
        throw new Error("Request Body isn't a JSON object");
      }

      for (const fieldName in bodyFields) {
        if (!bodyFields.hasOwnProperty(fieldName)) {
          continue;
        }

        const fieldValue = bodyFields[fieldName];

        encodedParams.forEach((param) => {
          if (param.name === fieldName) {
            switch (param.location) {
              case "header":
                headers[param.name] = fieldValue;
                break;
              case "query":
                addQuery(param.name, fieldValue);
                break;
              case "body":
                newBody[param.name] = fieldValue;
                break;
            }
          }
        });
      }
    } catch (e) {
      console.error("Unable to parse body: ", e);
      // but continue anyway
    }
  }

  let hasBody = false;
  const svcEncoding = apiEncoding.services.find((s) => s.name === rpc.service_name);
  const rpcEncoding = svcEncoding!.rpcs.find((r) => r.name === rpc.name);
  const reqEncoding = rpcEncoding!.all_request_encodings.find((r) =>
    r.http_methods.includes(method)
  );
  const headerParams = reqEncoding!.header_parameters ?? [];
  const queryParams = reqEncoding!.query_parameters ?? [];
  const bodyParams = reqEncoding!.body_parameters ?? [];
  const params = [...headerParams, ...queryParams, ...bodyParams];
  if (params.length) {
    hasBody = !!bodyParams.length;
    processStruct(params, reqBody);
  }
  const { authorization: auth } = apiEncoding;
  if (auth !== null && (auth.query_parameters?.length || auth.header_parameters?.length)) {
    const queryParams = auth.query_parameters ?? [];
    const headerParams = auth.header_parameters ?? [];
    processStruct([...queryParams, ...headerParams], authBody);
  }

  reqBody = JSON.stringify(newBody);

  const defaultMethod = reqEncoding!.http_methods[0];
  let cmd = "curl ";

  if (method !== defaultMethod && method !== "*") {
    cmd += `-X ${method} `;
  } else if (method === "*" && defaultMethod !== "GET") {
    // If we have a wildcard endpoint we use the default method,
    // unless it's GET in which case it's already implied.
    cmd += `-X ${defaultMethod}`;
  }

  cmd += `'http://${addr ?? "localhost:4000"}${path}${queryString}'`;

  for (const header in headers) {
    cmd += ` -H '${header}: ${headers[header]}'`;
  }

  if (hasBody) {
    cmd += ` -d '${reqBody}'`;
  }
  copyToClipboard(cmd);
  return cmd;
};
