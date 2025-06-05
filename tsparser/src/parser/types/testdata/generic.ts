// Basic interface with generic param
export interface Interface<T> {
  foo: T;
}

// Basic interface with generic param with default type
export interface InterfaceDefault<T = string> {
  foo: T;
}

// Interface with multiple generic param with default type
export interface InterfaceDefaultMulti<
  A,
  B,
  C,
  D = string,
  E = number,
  F = boolean
> {
  foo: A;
  bar: B;
  baz: C;
  qux: D;
  quux: E;
  corge: F;
}

export type X = Interface<string>;
export type Y = InterfaceDefault;
export type Z = InterfaceDefault<boolean>;

export type M1 = InterfaceDefaultMulti<object, undefined, null>;
export type M2 = InterfaceDefaultMulti<object, undefined, null, bigint>;
export type M3 = InterfaceDefaultMulti<object, undefined, null, bigint, false>;
export type M4 = InterfaceDefaultMulti<
  object,
  undefined,
  null,
  bigint,
  false,
  "literal"
>;
