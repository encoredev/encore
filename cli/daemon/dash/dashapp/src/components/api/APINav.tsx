import React, { FC, FunctionComponent, useState } from "react";
import { icons } from "~c/icons";
import { APIMeta, Service } from "~c/api/api";

interface Props {
  meta: APIMeta | null;
}

const APINav: FunctionComponent<Props> = ({ meta }) => {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 p-4 text-sm">
      {meta &&
        meta.svcs.map((svc, i) => (
          <div key={svc.name}>
            <SvcMenu svc={svc} />
          </div>
        ))}
    </div>
  );
};

const SvcMenu: FC<{ svc: Service }> = ({ svc }) => {
  const [contracted, setContracted] = useState(false);

  return (
    <div className="text-xs">
      <button
        type="button"
        onClick={() => setContracted(!contracted)}
        className="text-gray-900 hover:text-gray-700 focus:outline-none flex w-full items-center px-2 py-1 text-left transition duration-150 ease-in-out"
      >
        <div className="flex flex-grow items-center">
          <div className="flex-grow font-semibold leading-5 text-white">
            {svc.name}
          </div>
          <div className="flex-shrink-0 text-white text-opacity-70">
            Service
          </div>
        </div>
        {contracted
          ? icons.chevronRight("flex-shrink-0 -mr-1 ml-2 h-3 w-3 opacity-70")
          : icons.chevronDown("flex-shrink-0 -mr-1 ml-2 h-3 w-3 opacity-70")}
      </button>

      {!contracted && (
        <div className="space-y-1">
          {svc.rpcs.map((rpc, j) => (
            <a
              key={j}
              href={`#${svc.name}.${rpc.name}`}
              className="text-gray-700 hover:bg-gray-100 hover:text-gray-900 focus:outline-none focus:bg-gray-100 focus:text-gray-900 block py-1 pl-4"
            >
              {rpc.name}
            </a>
          ))}
        </div>
      )}
    </div>
  );
};

export default APINav;
