type durationUnit = "ns" | "µs" | "ms" | "s" | "m" | "h";
type durationComponent = `${number}${durationUnit}`;

/**
 * A duration is a string representing a length of time.
 */
export type DurationString =
  | durationComponent
  | `${durationComponent}${durationComponent}`
  | `${durationComponent} ${durationComponent}`;
