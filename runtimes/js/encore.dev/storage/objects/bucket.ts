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

  async *list(options: ListOptions): AsyncGenerator<ObjectAttrs> {
    const iter = await this.impl.list();
    while (true) {
      const attrs = await iter.next();
      if (attrs === null) {
        break;
      }
      yield attrs;
    }
  }

  /**
   * Returns whether the object exists in the bucket.
   * Throws an error on network failure.
   */
  async exists(name: string): Promise<boolean> {
    const impl = this.impl.object(name);
    return impl.exists();
  }

  /**
   * Returns the object's attributes.
   * Throws an error if the object does not exist.
   */
  async attrs(name: string): Promise<ObjectAttrs> {
    const impl = this.impl.object(name);
    return impl.attrs();
  }

  /**
   * Uploads an object to the bucket.
   */
  async upload(name: string, data: Buffer, options?: UploadOptions): Promise<ObjectAttrs> {
    const impl = this.impl.object(name);
    return impl.upload(data, options);
  }

  /**
   * Downloads an object from the bucket and returns its contents.
   */
  async download(name: string): Promise<Buffer> {
    const impl = this.impl.object(name);
    return impl.downloadAll();
  }

  /**
   * Removes an object from the bucket.
   * Throws an error on network failure.
   */
  async remove(name: string): Promise<void> {
    const impl = this.impl.object(name);
    return impl.delete();
  }
}

export interface ListOptions {
}

export interface ObjectAttrs {
  name: string;
  size: number;
  version: string;
  contentType?: string;
}

export interface UploadOptions {
  contentType?: string;
  preconditions?: {
    notExists?: boolean;
  },
}
