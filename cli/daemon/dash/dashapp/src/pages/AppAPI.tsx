import React, { FunctionComponent } from "react";
import { useParams } from "react-router-dom";
import AppAPI from "~c/app/AppAPI";
import Nav from "~c/Nav";
import { useConn } from "~lib/ctx";

const API: FunctionComponent = (props) => {
  const conn = useConn();
  const { appID } = useParams<{ appID: string }>();

  return (
    <>
      <Nav />

      <section className="bg-gray-200 flex flex-grow flex-col items-center">
        <div className="mt-6 w-full px-4 md:px-10">
          <AppAPI appID={appID!} conn={conn} />
        </div>
      </section>
    </>
  );
};

export default API;
