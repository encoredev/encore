export type MyType<T, K extends keyof T> = Omit<T, K> & {};
export type X = MyType<{ foo: string; bar: string; baz: string }, "bar">;

export type X2 = Omit<{ foo: string; bar: string; baz: string }, "bar"> & {};
