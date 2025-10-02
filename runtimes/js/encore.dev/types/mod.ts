import * as runtime from "../internal/runtime/mod";

export type ToDecimal = string | number | bigint;

/**
 * A decimal type that can hold values with arbitrary precision.
 * Unlike JavaScript's native number type, this can accurately represent
 * decimal values without floating-point precision errors.
 */
export class Decimal {
  private impl: runtime.Decimal;

  constructor(value: ToDecimal) {
    this.impl = new runtime.Decimal(String(value));
  }

  private static fromImpl(impl: runtime.Decimal): Decimal {
    const d = Object.create(Decimal.prototype);
    d.impl = impl;
    return d;
  }

  private toImpl(value: Decimal | ToDecimal): runtime.Decimal {
    return value instanceof Decimal
      ? value.impl
      : new runtime.Decimal(String(value));
  }

  add(d: Decimal | ToDecimal): Decimal {
    return Decimal.fromImpl(this.impl.add(this.toImpl(d)));
  }

  sub(d: Decimal | ToDecimal): Decimal {
    return Decimal.fromImpl(this.impl.sub(this.toImpl(d)));
  }

  mul(d: Decimal | ToDecimal): Decimal {
    return Decimal.fromImpl(this.impl.mul(this.toImpl(d)));
  }

  div(d: Decimal | ToDecimal): Decimal {
    return Decimal.fromImpl(this.impl.div(this.toImpl(d)));
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
