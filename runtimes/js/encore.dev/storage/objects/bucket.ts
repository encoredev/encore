import * as runtime from "../../internal/runtime/mod";
import { StringLiteral } from "../../internal/utils/constraints";

export interface BucketConfig {
  public?: boolean;
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
  constructor(name: string, cfg?: BucketConfig) {
    this.impl = runtime.RT.bucket(name);
  }

  /**
   * Reference an existing bucket by name.
   * To create a new storage bucket, use `new StorageBucket(...)` instead.
   */
  static named<name extends string>(name: StringLiteral<name>): Bucket {
    return new Bucket(name, {});
  }

  object(name: string): ObjectHandle {
    const impl = this.impl.object(name);
    return new ObjectHandle(impl, this, name);
  }
}

export class ObjectHandle {
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

  /**
   * Returns the object's attributes.
   * Throws an error if the object does not exist.
   */
  async attrs(): Promise<ObjectAttrs> {
    return this.impl.attrs();
  }

  async upload(data: Buffer, options?: UploadOptions): Promise<ObjectAttrs> {
    return this.impl.upload(data, options);
  }

  async download(): Promise<Buffer> {
    return this.impl.downloadAll();
  }
}

export interface ObjectAttrs {
  name: string;
  contentType?: string;
  size: number;
  version: string;
}

export interface UploadOptions {
  contentType?: string;
  preconditions?: {
    notExists?: boolean;
  },
}
