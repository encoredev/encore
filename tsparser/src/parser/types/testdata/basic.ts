export interface MyInterface {
  foo: string;
  bar: string;
  baz: string;
}
export type Optional3<MY_T, MY_K extends keyof MY_T> = Omit<MY_T, MY_K> &
  Partial<Pick<MY_T, MY_K>>;

export type X3 = Optional3<MyInterface, "bar">;
