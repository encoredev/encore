export type Generic1<T> = {
    cond: T extends string ? "literal" : number;
}

export type Generic2<T> = {
    value: T;
    cond: T extends string ? "literal" : number;
}

export type Concrete1 = {
    one: Generic1<string>;
    two: Generic1<"test">;
    three: Generic2<null>;
    four: Generic2<Generic1<boolean>>;
}
