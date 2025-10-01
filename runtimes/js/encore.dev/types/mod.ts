import * as runtime from "../internal/runtime/mod";

/**
 * A decimal type that can hold values with arbitrary precision.
 * Unlike JavaScript's native number type, this can accurately represent
 * decimal values without floating-point precision errors.
 */
export class Decimal {
  private impl: runtime.Decimal;

  constructor(value: string | number) {
    this.impl = new runtime.Decimal(
      typeof value === "number" ? value.toString() : value
    );
  }

  get value(): string {
    return this.impl.toString();
  }

  toJSON(): string {
    return this.impl.toString();
  }
  toString(): string {
    return this.impl.toString();
  }

  [Symbol.toPrimitive](hint: string) {
    if (hint === "number") {
      return +this.value;
    }

    return this.value;
  }

  private get __encore_decimal(): boolean {
    return true;
  }
}

runtime.RT.registerTypeConstructor(
  Decimal.name,
  (val: string | number) => new Decimal(val)
);
