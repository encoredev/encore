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

      <section className="flex flex-grow flex-col items-center">
        <AppAPI appID={appID!} conn={conn} />
      </section>
    </>
  );
};

export default API;
