// Basic return type
export type Ret = {
  id: number;
  name: string;
  description: string;
};

// Simple function with explicit return type
function HelloWorld(a: string, b: number): Ret {
  return {
    id: 1,
    name: "Hello",
    description: "Hello description",
  };
}

// Call expression - typeof should resolve to Ret
export const x = HelloWorld("hello", 5);
export type y = typeof x;

// Function with optional parameters
function withOptional(a: string, b?: number): string {
  return a;
}

export const optResult = withOptional("test");
export type OptType = typeof optResult;

// Function with rest parameters
function withRest(a: string, ...rest: number[]): boolean {
  return true;
}

export const restResult = withRest("test", 1, 2, 3);
export type RestType = typeof restResult;

// Function with destructured object parameter
function withObjParam({ x, y }: { x: number; y: number }): number {
  return x + y;
}

export const objResult = withObjParam({ x: 1, y: 2 });
export type ObjType = typeof objResult;

// Function with destructured array parameter
function withArrayParam([a, b]: [string, string]): string {
  return a + b;
}

export const arrResult = withArrayParam(["a", "b"]);
export type ArrType = typeof arrResult;

// Nested function calls
function getNumber(): number {
  return 42;
}

function useNumber(n: number): string {
  return "result";
}

export const nested = useNumber(getNumber());
export type NestedType = typeof nested;
