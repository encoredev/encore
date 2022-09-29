import React, { FunctionComponent, useEffect, useState } from "react";
import { Link, matchPath, useLocation, useParams } from "react-router-dom";
import { useConn } from "~lib/ctx";
import logo from "../logo.svg";
import wordmark from "../wordmark.svg";

interface NavProps {
  withoutLinks?: boolean;
}

const menuItems: {
  href: string;
  name: string;
  external?: boolean;
  badge?: string;
}[] = [
  { href: "", name: "Requests" },
  { href: "/api", name: "API Docs" },
  { href: "/flow", name: "Flow", badge: "New!" },
  { href: "https://encore.dev/docs", name: "Encore Docs", external: true },
];

const Nav: FunctionComponent<NavProps> = (props) => {
  const { appID } = useParams<{ appID: string }>();
  const [menuOpen, setMenuOpen] = useState(false);
  const [appsOpen, setAppsOpen] = useState(false);

  return (
    <nav className="bg-black">
      {appsOpen && <div className="absolute inset-0 z-10" onClick={() => setAppsOpen(false)} />}
      <div className="mx-auto px-4 md:px-10">
        <div className="flex h-nav-bar items-center justify-between">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <img
                className={`h-8 ${props.withoutLinks ? "hidden" : "hidden md:inline-block"}`}
                src={logo}
                alt="Encore Logo"
              />
              <img
                className={`h-8 ${props.withoutLinks ? "inline-block" : "inline-block md:hidden"}`}
                src={wordmark}
                alt="Encore Logo"
              />
            </div>

            <div className="hidden md:block">
              <div className="ml-10 flex items-baseline space-x-4">
                {!props.withoutLinks &&
                  menuItems
                    .filter((it) => !it.external)
                    .map((it) => {
                      const as = `/${appID}${it.href}`;
                      const { pathname } = useLocation();
                      const isSelected = !!matchPath({ path: "/:appID" + it.href }, pathname);
                      return (
                        <div key={it.name} className="flex items-center">
                          <Link
                            to={as}
                            className={`flex items-center rounded-sm px-3 py-2 text-lead-xxs text-white focus:outline-none hover:bg-white hover:bg-opacity-10 ${
                              isSelected ? "bg-white bg-opacity-10" : ""
                            }`}
                          >
                            {it.name}
                            {it.badge && (
                              <div
                                className="ml-2 flex items-center justify-center rounded px-1"
                                style={{
                                  fontSize: "10px",
                                  height: "13px",
                                  background: "#B3D77E",
                                  color: "#111111",
                                }}
                              >
                                <span>{it.badge}</span>
                              </div>
                            )}
                          </Link>
                        </div>
                      );
                    })}
              </div>
            </div>
          </div>

          {!props.withoutLinks && (
            <>
              <div className="absolute inset-y-0 right-0 hidden items-center pr-2 sm:static sm:inset-auto sm:ml-6 sm:pr-0 md:flex">
                <div className="ml-10 flex items-baseline space-x-4">
                  {menuItems
                    .filter((it) => it.external)
                    .map((it) => {
                      return (
                        <a
                          key={it.href}
                          href={it.href}
                          target="_blank"
                          className="rounded-sm px-3 py-2 text-lead-xxs text-white focus:outline-none hover:bg-white hover:bg-opacity-10"
                        >
                          {it.name}&nbsp;
                          <svg
                            className="inline-block h-4 w-4 pb-0.5"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                            xmlns="http://www.w3.org/2000/svg"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth="2"
                              d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                            ></path>
                          </svg>
                        </a>
                      );
                    })}
                </div>
                {/* <-- App dropdown --> */}
                <div className="ml-3">
                  <AppDropdown appID={appID!} open={appsOpen} setOpen={setAppsOpen} />
                </div>
              </div>

              <div className="-mr-2 flex md:hidden">
                <button
                  onClick={() => setMenuOpen(!menuOpen)}
                  className="inline-flex items-center justify-center rounded-md p-2 text-white focus:outline-none hover:bg-white hover:bg-opacity-10"
                >
                  <svg
                    className={`${menuOpen ? "hidden" : "block"} h-6 w-6`}
                    stroke="currentColor"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="2"
                      d="M4 6h16M4 12h16M4 18h16"
                    />
                  </svg>
                  <svg
                    className={`${menuOpen ? "block" : "hidden"} h-6 w-6`}
                    stroke="currentColor"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="2"
                      d="M6 18L18 6M6 6l12 12"
                    />
                  </svg>
                </button>
              </div>
            </>
          )}
        </div>
      </div>

      {!props.withoutLinks && (
        <div className={`${menuOpen ? "block" : "hidden"} md:hidden`}>
          <div className="space-y-1 px-1 pt-2 pb-3 sm:px-3">
            {menuItems.map((it) => {
              if (it.external) {
                return (
                  <a
                    key={it.href}
                    href={it.href}
                    target="_blank"
                    className="block rounded-md px-2 py-2 text-base font-medium text-white focus:outline-none hover:bg-white hover:bg-opacity-10"
                  >
                    {it.name}&nbsp;
                    <svg
                      className="inline-block h-4 w-4 pb-0.5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      xmlns="http://www.w3.org/2000/svg"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth="2"
                        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                      ></path>
                    </svg>
                  </a>
                );
              } else {
                const as = `/${appID}${it.href}`;
                const { pathname } = useLocation();
                const isSelected = !!matchPath({ path: "/:appID" + it.href }, pathname);
                return (
                  <Link
                    key={it.name}
                    to={as}
                    className={`block rounded-md px-2 py-2 text-base font-medium text-white ${
                      isSelected ? "bg-white bg-opacity-10" : ""
                    } hover:bg-white hover:bg-opacity-10`}
                  >
                    {it.name}
                  </Link>
                );
              }
            })}
          </div>
        </div>
      )}
    </nav>
  );
};

export default Nav;

interface AppDropdownProps {
  appID: string;
  open: boolean;
  setOpen: (open: boolean) => void;
}

const AppDropdown: FunctionComponent<AppDropdownProps> = (props): JSX.Element => {
  interface app {
    id: string;
    name: string;
  }

  const [apps, setApps] = useState<app[] | undefined>(undefined);
  const appName = apps?.find((a) => a.id === props.appID)?.name;
  const conn = useConn();

  useEffect(() => {
    conn.request("list-apps").then((apps) => setApps(apps as app[]));
  }, [props.open]);

  return (
    <>
      <div className="relative inline-block text-left">
        <div>
          <button
            type="button"
            className="inline-flex justify-center rounded-sm px-3 py-2 text-sm text-lead-xxs text-white focus:outline-none hover:bg-white hover:bg-opacity-10"
            // className="leading-5 transition duration-150 ease-in-out hover:text-white active:text-white"
            id="app-menu"
            aria-haspopup="true"
            aria-expanded="true"
            onClick={() => props.setOpen(!props.open)}
          >
            {appName ?? "Loading..."}
            <svg
              className="-mr-1 ml-2 h-5 w-5"
              fill="none"
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
            </svg>
          </button>
        </div>

        {props.open && (
          <div className="absolute right-0 z-20 mt-2 w-56 origin-top-right">
            <div
              className="brand-shadow bg-white"
              role="menu"
              aria-orientation="vertical"
              aria-labelledby="app-menu"
            >
              <div className="py-1">
                {apps !== undefined ? (
                  <>
                    <div className="px-2 py-1 font-mono text-xs uppercase">Running Apps</div>
                    {apps.map((app) => (
                      <Link
                        key={app.id}
                        to={"/" + app.id}
                        className="block px-2 py-2 text-sm hover:bg-black hover:text-white"
                        role="menuitem"
                        onClick={() => props.setOpen(false)}
                      >
                        <div className="truncate leading-4">{app.name}</div>
                      </Link>
                    ))}
                  </>
                ) : (
                  <span>Loading...</span>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </>
  );
};
