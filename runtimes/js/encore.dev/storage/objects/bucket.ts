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

  async *list(options: ListOptions): AsyncGenerator<ListEntry> {
    const iter = await this.impl.list();
    while (true) {
      const entry = await iter.next();
      if (entry === null) {
        break;
      }
      yield entry;
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
  async attrs(name: string, options?: AttrsOptions): Promise<ObjectAttrs> {
    const impl = this.impl.object(name);
    return impl.attrs(options);
  }

  /**
   * Uploads an object to the bucket.
   */
  async upload(name: string, data: Buffer, options?: UploadOptions): Promise<void> {
    const impl = this.impl.object(name);
    return impl.upload(data, options);
  }

  /**
   * Downloads an object from the bucket and returns its contents.
   */
  async download(name: string, options?: DownloadOptions): Promise<Buffer> {
    const impl = this.impl.object(name);
    return impl.downloadAll(options);
  }

  /**
   * Removes an object from the bucket.
   * Throws an error on network failure.
   */
  async remove(name: string, options?: DeleteOptions): Promise<void> {
    const impl = this.impl.object(name);
    return impl.delete(options);
  }
}

export interface ListOptions {
  /**
   * Only include objects with this prefix in the listing.
   * If unset, all objects are included.
  */
  prefix?: string;

  /** Maximum number of objects to return. Defaults to no limit. */
  limit?: number;
}

export interface AttrsOptions {
  /**
   * The object version to retrieve attributes for.
   * Defaults to the lastest version if unset.
   *
   * If bucket versioning is not enabled, this option is ignored.
   */
  version?: string;
}

export interface DeleteOptions {
  /**
   * The object version to delete.
   * Defaults to the lastest version if unset.
   *
   * If bucket versioning is not enabled, this option is ignored.
   */
  version?: string;
}

export interface DownloadOptions {
  /**
   * The object version to download.
   * Defaults to the lastest version if unset.
   *
   * If bucket versioning is not enabled, this option is ignored.
   */
  version?: string;
}

export interface ObjectAttrs {
  name: string;
  size: number;
  version: string;
  etag: string;
  contentType?: string;
}

export interface ListEntry {
  name: string;
  size: number;
  etag: string;
}

export interface UploadOptions {
  contentType?: string;
  preconditions?: {
    notExists?: boolean;
  },
}
