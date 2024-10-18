import { getCurrentRequest } from "../../internal/reqtrack/mod";
import * as runtime from "../../internal/runtime/mod";
import { StringLiteral } from "../../internal/utils/constraints";

export interface BucketConfig {
}

/**
 * Defines a new Object Storage bucket infrastructure resource.
 */
export class Bucket {
  impl: runtime.Bucket;

  /**
   * Creates a new bucket with the given name and configuration
   */
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  constructor(name: string, cfg: BucketConfig) {
    this.impl = runtime.RT.bucket(name);
  }

  /**
   * Reference an existing bucket by name.
   * To create a new storage bucket, use `new StorageBucket(...)` instead.
   */
  static named<name extends string>(name: StringLiteral<name>): Bucket {
    return new Bucket(name, {});
  }

  object(name: string): BucketObject {
    const impl = this.impl.object(name);
    return new BucketObject(impl, this, name);
  }
}

export class BucketObject {
  private impl: runtime.BucketObject;
  public readonly bucket: Bucket;
  public readonly name: string;

  constructor(impl: runtime.BucketObject, bucket: Bucket, name: string) {
    this.impl = impl;
    this.bucket = bucket;
    this.name = name;
  }

  /**
   * Returns whether the object exists in the bucket.
   * Throws an error on network failure.
   */
  async exists(): Promise<boolean> {
    return this.impl.exists();
  }
}
