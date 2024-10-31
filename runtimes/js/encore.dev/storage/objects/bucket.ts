import { getCurrentRequest } from "../../internal/reqtrack/mod";
import * as runtime from "../../internal/runtime/mod";
import { StringLiteral } from "../../internal/utils/constraints";

export interface BucketConfig {
  /**
   * Whether to enable versioning of the objects in the bucket.
   * Defaults to false if unset.
   */
  versioned?: boolean;
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
    const source = getCurrentRequest();
    const iter = await this.impl.list(options, source);
    while (true) {
      const entry = await iter.next();
      if (entry === null) {
        iter.markDone();
        break;
      }
      yield entry;
    }
  }

  /**
   * Returns whether the object exists in the bucket.
   * Throws an error on network failure.
   */
  async exists(name: string, options?: ExistsOptions): Promise<boolean> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    return impl.exists(options, source);
  }

  /**
   * Returns the object's attributes.
   * Throws an error if the object does not exist.
   */
  async attrs(name: string, options?: AttrsOptions): Promise<ObjectAttrs> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    return impl.attrs(options, source);
  }

  /**
   * Uploads an object to the bucket.
   */
  async upload(name: string, data: Buffer, options?: UploadOptions): Promise<ObjectAttrs> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    return impl.upload(data, options, source);
  }

  /**
   * Downloads an object from the bucket and returns its contents.
   */
  async download(name: string, options?: DownloadOptions): Promise<Buffer> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    return impl.downloadAll(options, source);
  }

  /**
   * Removes an object from the bucket.
   * Throws an error on network failure.
   */
  async remove(name: string, options?: DeleteOptions): Promise<void> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    const _ = await impl.delete(options, source);
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

export interface ExistsOptions {
  /**
   * The object version to check for existence.
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
  /** The version of the object, if bucket versioning is enabled. */
  version?: string;
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
