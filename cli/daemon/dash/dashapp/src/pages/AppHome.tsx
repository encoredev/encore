import React, { FunctionComponent } from 'react'
import { useParams } from 'react-router-dom'
import AppLogs from '~c/app/AppLogs'
import AppTraces from '~c/app/AppTraces'
import Nav from '~c/Nav'
import { useConn } from '~lib/ctx'

const AppHome: FunctionComponent = (props) => {
  const { appID } = useParams<{appID: string}>()
  const conn = useConn()

  return (
    <>
      <Nav />

      <section className="bg-gray-200 flex-grow flex flex-col items-center">
        <div className="w-full mt-6 px-10">
          <div className="flex items-stretch">
            <div className="flex-1 min-w-0 rounded-lg overflow-hidden mr-8">
              <AppTraces appID={appID} conn={conn} />
            </div>
            <div className="flex-1 min-w-0 rounded-lg overflow-hidden h-96">
              <AppLogs appID={appID} conn={conn} />
            </div>
          </div>
        </div>
      </section>
    </>
  )
}

export default AppHome