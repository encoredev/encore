// Mapped types with 'as' clause for key remapping

export interface Person {
  name: string;
  age: number;
  kind: string;
}

// Test 1: Filter out properties using never
export type RemoveKind<T> = {
  [K in keyof T as K extends "kind" ? never : K]: T[K]
}
export type PersonNoKind = RemoveKind<Person>;

// Test 2: Identity mapping
export type Identity<T> = {
  [K in keyof T as K]: T[K]
}
export type IdentityPerson = Identity<Person>;

// Test 3: Filter based on value type
export type OnlyStrings<T> = {
  [K in keyof T as T[K] extends string ? K : never]: T[K]
}
export type PersonStrings = OnlyStrings<Person>;
