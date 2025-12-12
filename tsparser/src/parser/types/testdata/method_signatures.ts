// Simple method signatures
export type SimpleMethod = {
  foo(): void;
  bar(): string;
};

// Methods with parameters
export type MethodsWithParams = {
  simple(a: string): void;
  multiple(a: string, b: number): boolean;
  withOptional(a?: string): void;
  withRest(...args: string[]): void;
};

// Methods with complex return types
export type ComplexReturns = {
  returnsObject(): { x: number };
  returnsArray(): string[];
  returnsUnion(): string | number;
};

// Optional methods
export type OptionalMethods = {
  required(): void;
  optional?(): void;
};

// Mixed properties and methods
export type MixedInterface = {
  prop: string;
  method(): number;
  optionalProp?: boolean;
  optionalMethod?(): string;
};

// Example mimicking the drizzle-orm pattern that was failing
export type Column<TDriverParam = unknown, TData = TDriverParam> = {
  mapFromDriverValue(value: TDriverParam): TData;
  mapToDriverValue(value: TData): TDriverParam;
};

// More complex example with multiple methods
export interface SqlDialect {
  escape(str: string): string;
  escapeId(id: string): string;
  buildQuery(sql: string, values: any[]): string;
}
