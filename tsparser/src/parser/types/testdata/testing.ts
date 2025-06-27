export interface MyInterface {
  foo: string;
  bar: string;
  baz: string;
}
export type Test1<T, K extends keyof T> = Omit<T, K>;
export type Bleh = Test1<MyInterface, "bar">;
