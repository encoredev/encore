import React, { useContext } from "react";
import JSONRPCConn from "./client/jsonrpc";

export const ConnContext = React.createContext<JSONRPCConn | undefined>(undefined);

export function useConn(): JSONRPCConn {
  return useContext(ConnContext)!;
}
