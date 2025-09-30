import * as runtime from "../internal/runtime/mod";

(runtime.Decimal.prototype as any)[Symbol.toPrimitive] = function (
  hint: string
) {
  if (hint === "number") {
    return +this.value;
  }

  return this.value;
};

/**
 * A decimal type that can hold values with arbitrary precision.
 * Unlike JavaScript's native number type, this can accurately represent
 * decimal values without floating-point precision errors.
 */
export class Decimal {
  private impl: runtime.Decimal;

  constructor(value: string) {
    this.impl = new runtime.Decimal(value);
  }

  toJSON(): string {
    return this.impl.toJSON();
  }

  get value(): string {
    return this.impl.value;
  }

  get __encore_decimal(): boolean {
    return true;
  }

  [Symbol.toPrimitive](hint: string) {
    if (hint === "number") {
      return +this.value;
    }

    return this.value;
  }
}
