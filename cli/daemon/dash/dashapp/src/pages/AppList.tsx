import React, {FunctionComponent, useEffect, useState} from 'react'
import {Link} from 'react-router-dom'
import {useConn} from '~lib/ctx'

const AppList: FunctionComponent = (props) => {
  const conn = useConn()
  const [apps, setApps] = useState<{id: string; name: string}[] | undefined>(undefined)
  useEffect(() => {
    conn.request("list-apps").then(apps => setApps(apps as {id: string; name: string}[]))
  }, [])

  return (
    <>
      <section className="bg-gray-200 flex-grow flex items-center justify-center">
        <div className="max-w-xl w-full bg-white overflow-hidden shadow-xl rounded-lg">
          <div className="border-b border-gray-200 px-4 py-5 sm:px-6 flex items-center">
            <h1 className="mr-auto text-xl font-bold">Your Apps</h1>
          </div>
          <div className="px-4 py-5 sm:p-6">
            {apps !== undefined ? (
              apps.map((app) =>
                <div key={app.id}>
                  <Link to={"/"+app.id} className="text-purple-600 hover:text-purple-700">{app.name}</Link>
                </div>
              )
            ) : <p>Loading...</p>
            }
          </div>
        </div>
      </section>
    </>
  )
}

export default AppList
