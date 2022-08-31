import React, { useEffect, useState } from "react";
import { BrowserRouter as Router, Route, Routes } from "react-router-dom";
import Client from "~lib/client/client";
import JSONRPCConn from "~lib/client/jsonrpc";
import AppList from "~p/AppList";
import AppHome from "~p/AppHome";
import { ConnContext } from "~lib/ctx";
import AppAPI from "~p/AppAPI";
import AppDiagram from "~p/AppDiagram";

function App() {
  const [conn, setConn] = useState<JSONRPCConn | undefined>(undefined);
  const [err, setErr] = useState<Error | undefined>(undefined);

  useEffect(() => {
    let hasConnection = false;
    const client = new Client();
    client.base
      .jsonrpc("/__encore")
      .then((conn) => {
        if (!hasConnection) {
          setConn(conn);
        } else {
          conn.close();
        }
      })
      .catch((err) => setErr(err));
    return () => {
      hasConnection = true;
    };
  }, []);

  if (err) return <div>Error: {err.message}</div>;
  if (!conn) return <div>Loading...</div>;

  return (
    <ConnContext.Provider value={conn}>
      <Router>
        <Routes>
          <Route path="/:appID/diagram" element={<AppDiagram />} />
          <Route path="/:appID/api" element={<AppAPI />} />
          <Route path="/:appID" element={<AppHome />} />
          <Route path="/" element={<AppList />} />
        </Routes>
      </Router>
    </ConnContext.Provider>
  );
}

export default App;
