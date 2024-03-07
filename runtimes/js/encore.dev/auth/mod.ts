export type AuthHandler<
  Params extends object,
  AuthData extends { userID: string },
> = ((params: Params) => Promise<AuthData | null>) & AuthHandlerBrand;

export type AuthHandlerBrand = { readonly __authHandlerBrand: unique symbol };

export function authHandler<
  Params extends object,
  AuthData extends { userID: string },
>(
  fn: (params: Params) => Promise<AuthData | null>,
): AuthHandler<Params, AuthData> {
  return fn as any;
}
