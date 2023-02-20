import React, { FunctionComponent, useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { useConn } from "~lib/ctx";
import { NotificationMsg } from "~lib/client/jsonrpc";
import { ProcessReload } from "~lib/client/client";
import { APIMeta } from "~c/api/api";
import { FlowDiagram } from "~c/FlowDiagram/FlowDiagram";

const Diagram: FunctionComponent = () => {
  const conn = useConn();
  const { appID } = useParams<{ appID: string }>();
  const [metaData, setMetaData] = useState<APIMeta>();

  useEffect(() => {
    conn.request("status", { appID }).then((status: any) => {
      if (status.meta) {
        setMetaData(status.meta);
      }
    });
    const onNotify = (msg: NotificationMsg) => {
      if (msg.method === "process/reload") {
        const data = msg.params as ProcessReload;
        if (data.appID === appID) {
          setMetaData(data.meta);
        }
      }
    };
    conn.on("notification", onNotify);
    return () => {
      conn.off("notification", onNotify);
    };
  }, []);

  return (
    <div className="h-full-minus-nav w-full">{metaData && <FlowDiagram metaData={metaData} />}</div>
  );
};

export default Diagram;
