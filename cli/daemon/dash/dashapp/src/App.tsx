import React, { FC, useEffect, useState } from "react";
import { BrowserRouter as Router, Navigate, Route, Routes, useParams } from "react-router-dom";
import Client from "~lib/client/client";
import JSONRPCConn from "~lib/client/jsonrpc";
import AppList from "~p/AppList";
import AppHome from "~p/AppHome";
import { ConnContext } from "~lib/ctx";
import AppAPI from "~p/AppAPI";
import AppDiagram from "~p/AppDiagram";
import { SnippetContent, SnippetPage } from "~p/SnippetPage";

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
          <Route path="/" element={<AppList />} />

          <Route path="/:appID">
            <Route index element={<Redirect to="requests" />} />

            <Route path="requests" element={<AppHome />} />

            <Route path="snippets" element={<SnippetPage />}>
              <Route path=":slug" element={<SnippetContent />} />
            </Route>

            <Route path="flow" element={<AppDiagram />} />

            <Route path="api" element={<AppAPI />} />
          </Route>
        </Routes>
      </Router>
    </ConnContext.Provider>
  );
}

export default App;

const Redirect: FC<{ to: string }> = ({ to }) => {
  const params = useParams<{ appID: string }>();
  return <Navigate to={`/${params.appID}/${to}`} replace />;
};
