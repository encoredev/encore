export interface Foo {
  foo: string | number | null;
  bar: number;
  optional?: boolean;
}

export interface Bar extends Foo {
  foo: string | null; // now never a number
  optional: boolean; // now required
  moo: string;
}

export interface Generic<T> {
  foo: T | null;
}

export interface ExtendGeneric extends Generic<string | number> {
  bar: string;
}

export interface MergeGeneric extends Generic<number> {
  foo: 5 | null;
}
