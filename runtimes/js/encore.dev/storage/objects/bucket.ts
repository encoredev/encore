import { getCurrentRequest } from "../../internal/reqtrack/mod";
import * as runtime from "../../internal/runtime/mod";
import { StringLiteral } from "../../internal/utils/constraints";

export interface BucketConfig {
}

/**
 * Defines a new Object Storage bucket infrastructure resource.
 */
export class Bucket {
  /**
   * Creates a new bucket with the given name and configuration
   */
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  constructor(name: string, cfg?: BucketConfig) {
    // this.impl = runtime.RT.sqlDatabase(name);
  }

  /**
   * Reference an existing bucket by name.
   * To create a new storage bucket, use `new StorageBucket(...)` instead.
   */
  static named<name extends string>(name: StringLiteral<name>): Bucket {
    return new Bucket(name);
  }

}
