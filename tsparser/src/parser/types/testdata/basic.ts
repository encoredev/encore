// Basic interface
export interface Interface {
    foo: string;
    bar: number;
    optional?: boolean;
}

// Utility types
export type Exclude1 = Exclude<keyof Interface, "foo">;
export type Pick1 = Pick<Interface, "foo">;
export type Pick2 = Pick<Interface, "foo" | "optional">;
export type Omit1 = Omit<Interface, "foo">;
export type Omit2 = Omit<Interface, "foo" | "bar">;

export type Partial1 = Partial<Interface>;

// Index signatures
export type Index = { [key: string]: boolean | number};

// Intersections
export type Intersect1 = {foo: string} & {bar: number};
export type Intersect2 = {foo: string} & {foo: "literal"};
export type Intersect3 = {foo: string} & {foo: number};
export type Intersect4 = {foo?: "optional"} & {foo: string};
export type Intersect5 = {a: string; b: string; c: string} & {a: any; b: unknown; c: never};

// Enums
export enum Enum1 {
    A,
    B,
    C,
    D = "foo",
    E = 5,
    F,
}
export type EnumFields = keyof typeof Enum1;
