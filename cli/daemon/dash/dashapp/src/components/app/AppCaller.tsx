import React, {FC, useEffect, useReducer, useState} from 'react'
import {APIMeta, RPC, Service} from '~c/api/api'
import RPCCaller from '~c/api/RPCCaller'
import {ProcessReload, ProcessStart} from '~lib/client/client'
import JSONRPCConn, {NotificationMsg} from '~lib/client/jsonrpc'
import Combobox, {ComboboxOptionsItem} from "~c/Combobox";

interface API {
  svc: Service;
  rpc: RPC;
  name: string;
}

const AppCaller: FC<{appID: string; conn: JSONRPCConn}> = ({appID, conn}) => {
  const [addr, setAddr] = useState("localhost:4000")
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
        setAddr(data.addr)
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
      if (resp.addr) { setAddr(resp.addr) }
      if (resp.meta) { dispatch({type: "meta", meta: resp.meta}) }
    })

    conn.on("notification", onNotify)
    return () => { conn.off("notification", onNotify) }
  }, [])

  if (!state.md || !state.selected) { return null }

  return (
    <div className="bg-white p-4">
      <Combobox
        label="API Endpoint"
        selectedItem={state.selected!}
        items={state.list!}
        onChange={(item: ComboboxOptionsItem) =>
          dispatch({ type: "select", name: item.name })
        }
      />
      <div className="mt-3">
        <RPCCaller conn={conn} appID={appID} md={state.md} svc={state.selected.svc} rpc={state.selected.rpc} addr={addr} />
      </div>
    </div>
  )
}

export default AppCaller
