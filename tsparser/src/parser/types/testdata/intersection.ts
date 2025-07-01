export type MyType<T, K extends keyof T> = Omit<T, K> & {};

// X1 and X2 should result in the same type
export type X1 = MyType<{ foo: string; bar: string; baz: string }, "bar">;
export type X2 = Omit<{ foo: string; bar: string; baz: string }, "bar"> & {};
