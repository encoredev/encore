import { Min, Max, MinLen, MaxLen, IsURL, IsEmail } from "encore.dev/validate";

export interface Params {
  // A number between 3 and 1000 (inclusive).
  foo: number & (Min<3> & Max<1000>);

  // A string between 5 and 20 characters long.
  bar: string & (MinLen<5> & MaxLen<20>);

  // A string that is either a URL or an email address.
  urlOrEmail: string & (IsURL | IsEmail);

  // An array of up to 10 email addresses.
  emails: Array<string & IsEmail> & MaxLen<10>;
}
