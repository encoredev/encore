import { Query, Header } from "encore.dev/api";
import { MinLen, Max } from "encore.dev/validate";

export interface Foo {
  a: Query<number>;
  b: Query<number> & Max<10>;
  c: Query<string> & MinLen<3>;
  d: Header<"X-Header"> & MinLen<3>;
}
