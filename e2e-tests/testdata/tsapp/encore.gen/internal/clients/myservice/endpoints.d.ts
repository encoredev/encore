import { CallOpts } from "encore.dev/api";

type Parameters<T> = T extends (...args: infer P) => unknown ? P : never;
type WithCallOpts<T extends (...args: any) => any> = (
  ...args: [...Parameters<T>, opts?: CallOpts]
) => ReturnType<T>;

import { hello as hello_handler } from "../../../../myservice/api.js";
declare const hello: WithCallOpts<typeof hello_handler>;
export { hello };

import { middlewareDemo as middlewareDemo_handler } from "../../../../myservice/api.js";
declare const middlewareDemo: WithCallOpts<typeof middlewareDemo_handler>;
export { middlewareDemo };

import { createUserViaService as createUserViaService_handler } from "../../../../myservice/api.js";
declare const createUserViaService: WithCallOpts<typeof createUserViaService_handler>;
export { createUserViaService };

import { getUsersViaService as getUsersViaService_handler } from "../../../../myservice/api.js";
declare const getUsersViaService: WithCallOpts<typeof getUsersViaService_handler>;
export { getUsersViaService };

import { createItem as createItem_handler } from "../../../../myservice/api.js";
declare const createItem: WithCallOpts<typeof createItem_handler>;
export { createItem };

import { getItem as getItem_handler } from "../../../../myservice/api.js";
declare const getItem: WithCallOpts<typeof getItem_handler>;
export { getItem };


