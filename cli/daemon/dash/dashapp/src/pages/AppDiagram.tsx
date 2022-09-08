import React, { FunctionComponent, useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import Nav from "~c/Nav";
import { useConn } from "~lib/ctx";
import { NotificationMsg } from "~lib/client/jsonrpc";
import { ProcessReload } from "~lib/client/client";
import { APIMeta } from "~c/api/api";
import FlowDiagram from "~c/FlowDiagram/FlowDiagram";

const Diagram: FunctionComponent = () => {
  const conn = useConn();
  const { appID } = useParams<{ appID: string }>();
  const [metaData, setMetaData] = useState<APIMeta>();

  // header + padding has a height of 110px
  const wrapperDivStyling: React.CSSProperties = {
    height: "calc(100vh - 110px)",
  };

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
    <>
      <Nav />
      <section className="flex flex-grow flex-col items-center bg-gray-200">
        <div className="mt-6 w-full px-4 md:px-10" style={wrapperDivStyling}>
          {metaData && <FlowDiagram metaData={metaData} />}
        </div>
      </section>
    </>
  );
};

export default Diagram;
