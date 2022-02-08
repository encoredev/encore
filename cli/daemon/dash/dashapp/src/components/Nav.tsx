import React, {FunctionComponent, useEffect, useState} from "react";
import {Link, useParams, useRouteMatch} from "react-router-dom"
import {useConn} from "~lib/ctx"
import logo from "../logo.svg"

const menuItems: {href: string; name: string}[] = [
  {href: "", name: "Requests"},
  {href: "/api", name: "API Docs"},
]

const Nav: FunctionComponent = (props) => {
  const { appID } = useParams<{appID: string}>()
  const [menuOpen, setMenuOpen] = useState(false)
  const [appsOpen, setAppsOpen] = useState(false)
  const route = useRouteMatch()

  return (
    <nav className="bg-gray-800">
      {appsOpen &&
        <div className="absolute inset-0 z-10" onClick={() => setAppsOpen(false)} />
      }
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          <div className="flex items-center">
            <div className="flex-shrink-0">
              <img className="h-8 w-8" src={logo} alt="Encore Logo" />
            </div>
            <div className="hidden md:block">
              <div className="ml-10 flex items-baseline space-x-4">
                {menuItems.map(it => {
                  const as = `/${appID}${it.href}`
                  const selected = route.path === ("/:appID"+it.href)
                  return (
                    <Link key={it.name} to={as}
                        className={`px-3 py-2 rounded-md text-sm font-medium ${selected ? "text-white bg-gray-600" : "text-gray-300 hover:text-white hover:bg-gray-700"} focus:outline-none focus:text-white focus:bg-gray-700`}>
                      {it.name}
                    </Link>
                  )
                })}
              </div>
            </div>
          </div>

          <div className="absolute inset-y-0 right-0 hidden md:flex items-center pr-2 sm:static sm:inset-auto sm:ml-6 sm:pr-0">
            {/* <-- App dropdown --> */}
            <div className="ml-3">
              <AppDropdown appID={appID} open={appsOpen} setOpen={setAppsOpen} />
            </div>
          </div>

          <div className="-mr-2 flex md:hidden">
            <button onClick={() => setMenuOpen(!menuOpen)} className="inline-flex items-center justify-center p-2 rounded-md text-gray-400 hover:text-white hover:bg-gray-700 focus:outline-none focus:bg-gray-700 focus:text-white">
              <svg className={`${menuOpen ? "hidden" : "block"} h-6 w-6`} stroke="currentColor" fill="none" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16M4 18h16" />
              </svg>
              <svg className={`${menuOpen ? "block" : "hidden"} h-6 w-6`} stroke="currentColor" fill="none" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      <div className={`${menuOpen ? "block" : "hidden"} md:hidden`}>
        <div className="px-2 pt-2 pb-3 space-y-1 sm:px-3">
          {menuItems.map(it => {
            const as = `/${appID}${it.href}`
            const selected = false // TODO
            return (
              <Link key={it.name} to={as}
                  className={`block px-3 py-2 rounded-md text-base font-medium ${selected ? "text-white bg-gray-900" : "text-gray-300 hover:text-white hover:bg-gray-700"} focus:outline-none focus:text-white focus:bg-gray-700`}>
                {it.name}
              </Link>
            )
          })}
        </div>

      </div>
    </nav>
  )
}

export default Nav

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
  const [apps, setApps] = useState<app[] | undefined>(undefined)
  const appName = apps?.find(a => a.id === props.appID)?.name
  const conn = useConn()

  useEffect(() => {
    conn.request("list-apps").then(apps => setApps(apps as app[]))
  }, [props.open])

  return (
    <>
      <div className="relative inline-block text-left">
        <div>
          <button type="button" className="inline-flex justify-center text-sm leading-5 font-medium text-gray-300 hover:text-white focus:outline-none active:text-white transition ease-in-out duration-150"
              id="app-menu" aria-haspopup="true" aria-expanded="true" onClick={() => props.setOpen(!props.open)}>
            {appName ?? "Loading..."}
            <svg className="h-5 w-5 -mr-1 ml-2" fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
              <path d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
            </svg>
          </button>
        </div>

        {props.open &&
          <div className="origin-top-right absolute right-0 mt-2 w-56 rounded-md shadow-lg z-20">
            <div className="rounded-md bg-white shadow-xs" role="menu" aria-orientation="vertical" aria-labelledby="app-menu">
              <div className="py-1">
                {apps !== undefined ? (
                  <>
                    <div className="font-bold px-2 py-1 text-xs text-gray-600">Running Apps</div>
                    {apps.map(app =>
                      <Link key={app.id} to={"/"+app.id}
                        className="block px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 hover:text-gray-900 focus:outline-none focus:bg-gray-100 focus:text-gray-900" role="menuitem"
                            onClick={(e) => props.setOpen(false)}>
                          <div className="truncate leading-4">{app.name}</div>
                      </Link>
                    )}
                  </>
                ) : (
                  <span>Loading...</span>
                )}
              </div>
            </div>
          </div>
        }
      </div>
    </>
  )
}
