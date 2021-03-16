import React, { FC, useEffect, useReducer, useState } from 'react'
import { APIMeta, RPC, Service } from '~c/api/api'
import RPCCaller from '~c/api/RPCCaller'
import { ProcessReload, ProcessStart } from '~lib/client/client'
import JSONRPCConn, { NotificationMsg } from '~lib/client/jsonrpc'

interface API {
  svc: Service;
  rpc: RPC;
  name: string;
}

const AppCaller: FC<{appID: string; conn: JSONRPCConn}> = ({appID, conn}) => {
  const [port, setPort] = useState(4060)
  interface State {
    md?: APIMeta;
    list?: API[];
    selected?: API;
  }
  function reducer(state: State, action: {type: "meta" | "select"; meta?: APIMeta; name?: string}): State {
    switch (action.type) {
      case "meta":
        // Recompute our API list
        const list: API[] = []
        const md = action.meta!
        md.svcs.forEach(svc => {
          svc.rpcs.forEach(rpc => {
            list.push({svc, rpc, name: `${svc.name}.${rpc.name}`})
          })
        })
        list.sort((a, b) => a.name.localeCompare(b.name))

        // Does the selected API still exist?
        const exists = state.selected ? list.findIndex(a => a.name === state.selected!.name) >= 0 : false
        const newSel = exists ? state.selected : list.length > 0 ? list[0] : undefined
        return {md: md, list: list, selected: newSel}
      case "select":
        const sel = state.list?.find(a => a.name === action.name)
        return {...state, selected: sel ?? state.selected}
    }
  }

  const [state, dispatch] = useReducer(reducer, {})
  const onNotify = (msg: NotificationMsg) => {
    if (msg.method === "process/start") {
      const data = msg.params as ProcessStart
      if (data.appID === appID) {
        setPort(data.port)
      }
    } else if (msg.method === "process/reload") {
      const data = msg.params as ProcessReload
      if (data.appID === appID) {
        dispatch({type: "meta", meta: data.meta})
      }
    }
  }

  useEffect(() => {
    conn.request("status", {appID}).then((resp: any) => {
      if (resp.port) { setPort(resp.port) }
      if (resp.meta) { dispatch({type: "meta", meta: resp.meta}) }
    })

    conn.on("notification", onNotify)
    return () => { conn.off("notification", onNotify) }
  }, [])

  if (!state.md || !state.selected) { return null }

  return (
    <div className="bg-white p-4">
      <div>
        <label htmlFor="endpoint" className="block text-sm font-medium text-gray-700">API Endpoint</label>
        <select id="endpoint" className="mt-1 block w-full pl-3 pr-10 py-2 text-base border-gray-300 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm rounded-md"
            value={state.selected!.name} onChange={(e) => dispatch({type: "select", name: e.target.value})}>
          {state.list!.map(a =>
            <option key={a.name} value={a.name}>{a.name}</option>
          )}
        </select>
      </div>
      <div className="mt-3">
        <RPCCaller conn={conn} appID={appID} md={state.md} svc={state.selected.svc} rpc={state.selected.rpc} port={port} />
      </div>
    </div>
  )
}

export default AppCaller