import React, { FC, useEffect, useState } from 'react'
import { APIMeta, RPC, Service } from '~c/api/api'
import RPCCaller from '~c/api/RPCCaller'
import { ProcessStart } from '~lib/client/client'
import JSONRPCConn, { NotificationMsg } from '~lib/client/jsonrpc'

interface API {
  svc: Service;
  rpc: RPC;
  name: string;
}

const AppCaller: FC<{appID: string; conn: JSONRPCConn}> = ({appID, conn}) => {
  const [meta, setMeta] = useState<APIMeta | undefined>(undefined)
  const [sel, setSel] = useState<API | undefined>(undefined)
  const [apis, setApis] = useState<API[]>([])

  const updateMeta = (md: APIMeta) => {
    const list: API[] = []
    md.svcs.forEach(svc => {
      svc.rpcs.forEach(rpc => {
        list.push({svc, rpc, name: `${svc.name}.${rpc.name}`})
      })
    })
    list.sort((a, b) => a.name.localeCompare(b.name))

    setMeta(md)
    setApis(list)
    const foundExisting = sel ? list.findIndex(a => a.name === sel.name) >= 0 : false
    if (!foundExisting) {
      setSel(list[0])
    }
  }

  const onNotify = (msg: NotificationMsg) => {
    if (msg.method === "process/start") {
      const data = msg.params as ProcessStart
      if (data.appID === appID) {
        updateMeta(data.meta)
      }
    }
  }

  useEffect(() => {
    conn.request("status", {appID}).then((resp: any) => {
      if (resp.meta) { updateMeta(resp.meta)}
    })

    conn.on("notification", onNotify)
    return () => { conn.off("notification", onNotify) }
  }, [])

  if (!meta || !sel) { return null }

  return (
    <div className="bg-white p-4">
      <div>
        <label htmlFor="endpoint" className="block text-sm font-medium text-gray-700">API Endpoint</label>
        <select id="endpoint" className="mt-1 block w-full pl-3 pr-10 py-2 text-base border-gray-300 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm rounded-md"
            value={sel.name} onChange={(e) => setSel(apis.find(a => a.name === e.target.value))}>
          {apis.map(a =>
            <option key={a.name} value={a.name}>{a.name}</option>
          )}
        </select>
      </div>
      <RPCCaller conn={conn} appID={appID} md={meta} svc={sel.svc} rpc={sel.rpc} />
    </div>
  )
}

export default AppCaller