import React, { FunctionComponent } from 'react'
import { useParams } from 'react-router-dom'
import AppCaller from '~c/app/AppCaller'
import AppTraces from '~c/app/AppTraces'
import Nav from '~c/Nav'
import { useConn } from '~lib/ctx'

const AppHome: FunctionComponent = (props) => {
  const { appID } = useParams<{appID: string}>()
  const conn = useConn()

  return (
    <>
      <Nav />

      <section className="bg-gray-200 flex-grow flex flex-col items-center py-6">
        <div className="w-full px-10">
          <div className="md:flex md:items-stretch">
            <div className="flex-1 min-w-0 md:mr-8">
              <h2 className="px-2 text-lg font-medium">API Explorer</h2>
              <div className="mt-2 rounded-lg overflow-hidden">
                <AppCaller appID={appID} conn={conn} />
              </div>
            </div>
            <div className="mt-4 md:mt-0 flex-1 min-w-0">
              <h2 className="px-2 text-lg font-medium">Traces</h2>
              <div className="mt-2 rounded-lg overflow-hidden">
                <AppTraces appID={appID} conn={conn} />
              </div>
            </div>
          </div>
        </div>
      </section>
    </>
  )
}

export default AppHome