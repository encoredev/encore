import React, { FunctionComponent } from "react";
import { useParams } from "react-router-dom";
import Nav from "~c/Nav";
import { useConn } from "~lib/ctx";
import AppDiagram from "~c/app/AppDiagram";

const Diagram: FunctionComponent = () => {
  const conn = useConn();
  const { appID } = useParams<{ appID: string }>();

  return (
    <>
      <Nav />
      <section className="flex flex-grow flex-col items-center bg-gray-200">
        <div className="mt-6 w-full px-4 md:px-10">
          <AppDiagram appID={appID!} conn={conn} />
        </div>
      </section>
    </>
  );
};

export default Diagram;
