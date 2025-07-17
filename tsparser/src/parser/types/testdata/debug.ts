export type MyPick<T, K extends keyof T> = {
  [P in K]: T[P];
};

export type MyExclude<T, U> = T extends U ? never : T;

export type MyOmit<T, K extends keyof any> = MyPick<T, MyExclude<keyof T, K>>;

export interface MyInterface {
  foo: string;
  bar: string;
  baz: string;
}

export type X = MyOmit<MyInterface, "foo">;
export type Y = MyPick<MyInterface, "foo">;
