const foo = {
  bar: "blah",
  nested: {
    baz: 55
  }
};

export type O = typeof foo;
export type S = typeof foo.bar;
export type N = typeof foo.nested.baz;
