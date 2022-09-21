import React, { FunctionComponent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { icons } from "~c/icons";
import { useConn } from "~lib/ctx";
import Nav from "~c/Nav";

const AppList: FunctionComponent = (props) => {
  const conn = useConn();
  const [apps, setApps] = useState<{ id: string; name: string }[] | undefined>(
    undefined
  );

  useEffect(() => {
    let ignore = false;

    async function fetchApps() {
      await conn.request("list-apps").then((apps) => {
        if (!ignore) {
          setApps(apps as { id: string; name: string }[]);
        }
      });
    }

    fetchApps();

    return () => {
      ignore = true;
    };
  }, []);

  return (
    <div className="h-screen overflow-hidden">
      <Nav withoutLinks />

      <div className="relative -mt-[100px] flex h-full w-full min-w-0 flex-col justify-center">
        <section className="flex flex-row justify-center">
          <div className="mr-0 flex-col px-4 sm:mr-24 sm:px-6">
            <h1 className="font-sans text-lead font-normal">
              Your applications
            </h1>

            <ol className="my-10 list-decimal space-y-2 list-brandient brandient-1">
              {apps !== undefined ? (
                apps.map((app) => (
                  <li key={app.id}>
                    <Link to={"/" + app.id}>
                      <a className="lead-base block font-mono">{app.name}</a>
                    </Link>
                  </li>
                ))
              ) : (
                <p>Loading...</p>
              )}
            </ol>
          </div>
          <div className="ml-10 hidden md:flex">
            <div className="w-60 self-center">
              <img src="/encore-patch-beginning.png" />
            </div>
          </div>
        </section>
      </div>
    </div>
  );
};

export default AppList;
