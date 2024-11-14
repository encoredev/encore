namespace Foo {
  export type Bar = string;

  export namespace Nested {
    export type Baz = number;
  }
};


export type T1 = Foo.Bar;
export type T2 = Foo.Nested.Baz;
