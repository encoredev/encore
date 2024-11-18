import { Min, Max, MinLen, MaxLen } from "encore.dev/validate";

export type Validate1 = number & Min<3>;
export type Validate2 = number & Min<3> & Max<5>;
export type Validate3 = number & Min<3> & Min<5>;
export type Validate4 = MinLen<3> & string & MaxLen<10>;
