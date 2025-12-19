// Basic call signature
export type BasicCallable = {
  (arg: string): number;
};

// Call signature with multiple parameters
export type MultiParamCallable = {
  (a: string, b: number): boolean;
};

// Call signature with optional parameters
export type OptionalCallable = {
  (required: string, optional?: number): void;
};

// Mixed: properties and call signature
export type MixedCallable = {
  prop: string;
  (arg: number): string;
};

// Call signature with rest parameters
export type RestCallable = {
  (first: string, ...rest: number[]): boolean;
};

// Interface with call signature
export interface CallableInterface {
  (x: number): string;
}

// Call signature with no parameters
export type NoParamCallable = {
  (): string;
};

// Overloaded call signatures
export type OverloadedCallable = {
  (x: string): number;
  (x: number): string;
  (x: boolean, y: number, z: string): void;
};

export const Callable: BasicCallable = (arg: string): number => {
  return 1;
};
const res = Callable("test");
export type Res = typeof res;
