import React, {useEffect, useRef, useState} from 'react'
import {BrowserRouter as Router, Route, Switch} from "react-router-dom";
import Client from '~lib/client/client';
import JSONRPCConn from '~lib/client/jsonrpc';
import AppList from '~p/AppList';
import AppHome from '~p/AppHome';
import {ConnContext} from '~lib/ctx';
import AppAPI from '~p/AppAPI';

function App() {
  const [conn, setConn] = useState<JSONRPCConn | undefined>(undefined)
  const [err, setErr] = useState<Error | undefined>(undefined)
  const mounted = useRef(true)

  useEffect(() => {
    const client = new Client()
    client.base.jsonrpc("/__encore").then(
      conn => mounted.current && setConn(conn)
    ).catch(err => mounted.current && setErr(err))
    return () => { conn?.close(); mounted.current = false }
  }, [])

  if (err) return <div>Error: {err.message}</div>
  if (!conn) return <div>Loading...</div>

  return (
    <ConnContext.Provider value={conn}>
      <Router>
        <Switch>
          <Route path="/:appID/api"><AppAPI /></Route>
          <Route path="/:appID"><AppHome /></Route>
          <Route path="/"><AppList /></Route>
        </Switch>
      </Router>
    </ConnContext.Provider>
  )
}

export default App
