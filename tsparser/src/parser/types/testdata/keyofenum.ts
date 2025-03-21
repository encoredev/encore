export type ObjectValues<T> = T[keyof T];

export const MY_ENUM = {
  VARIANT_1: "VARIANT_1",
  VARIANT_2: "VARIANT_2",
  VARIANT_3: "VARIANT_3"
} as const;

export type MyEnum = ObjectValues<typeof MY_ENUM>;
