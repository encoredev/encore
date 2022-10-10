import React, { FunctionComponent } from "react";
import { useParams } from "react-router-dom";
import AppCaller from "~c/app/AppCaller";
import AppTraces from "~c/app/AppTraces";
import Nav from "~c/Nav";
import { useConn } from "~lib/ctx";

const AppHome: FunctionComponent = (props) => {
  const { appID } = useParams<{ appID: string }>();
  const conn = useConn();

  return (
    <>
      <Nav />

      <section className="bg-gray-200 flex flex-grow flex-col items-center py-6">
        <div className="w-full px-4 md:px-10">
          <div className="md:flex md:items-stretch">
            <div className="min-w-0 flex-1 md:mr-8">
              <h2 className="text-lg font-medium">API Explorer</h2>
              <div className="mt-2 rounded-lg">
                <AppCaller key={appID} appID={appID!} conn={conn} />
              </div>
            </div>
            <div className="mt-4 min-w-0 flex-1 md:mt-0">
              <h2 className="text-lg font-medium">Traces</h2>
              <div className="mt-2 overflow-hidden rounded-lg">
                <AppTraces key={appID} appID={appID!} conn={conn} />
              </div>
            </div>
          </div>
        </div>
      </section>
    </>
  );
};

export default AppHome;
