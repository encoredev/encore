import { getCurrentRequest } from "../../internal/reqtrack/mod";
import * as runtime from "../../internal/runtime/mod";
import { StringLiteral } from "../../internal/utils/constraints";
import { unwrapErr } from "./error";
import { BucketPerms, Uploader, SignedUploader, Downloader, SignedDownloader, Attrser, Lister, Remover, PublicUrler } from "./refs";

export interface BucketConfig {
  /**
   * Whether the objects in the bucket should be publicly
   * accessible, via CDN. Defaults to false if unset.
  */
  public?: boolean;

  /**
   * Whether to enable versioning of the objects in the bucket.
   * Defaults to false if unset.
   */
  versioned?: boolean;
}

/**
 * Defines a new Object Storage bucket infrastructure resource.
 */
export class Bucket extends BucketPerms implements Uploader, SignedUploader, Downloader, SignedDownloader, Attrser, Lister, Remover, PublicUrler {
  impl: runtime.Bucket;

  /**
   * Creates a new bucket with the given name and configuration
   */
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  constructor(name: string, cfg?: BucketConfig) {
    super();
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
    const iter = unwrapErr(await this.impl.list(options, source));
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
    const res = await impl.exists(options, source);
    return unwrapErr(res);
  }

  /**
   * Returns the object's attributes.
   * Throws an error if the object does not exist.
   */
  async attrs(name: string, options?: AttrsOptions): Promise<ObjectAttrs> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    const res = await impl.attrs(options, source);
    return unwrapErr(res);
  }

  /**
   * Uploads an object to the bucket.
   */
  async upload(name: string, data: Buffer, options?: UploadOptions): Promise<ObjectAttrs> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    const res = await impl.upload(data, options, source);
    return unwrapErr(res);
  }

  /**
   * Generate an external URL to allow uploading an object to the bucket.
   * 
   * Anyone with possession of the URL can write to the given object name
   * without any additional auth.
   */
  async signedUploadUrl(name: string, options?: UploadUrlOptions): Promise<SignedUploadUrl> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    const res = await impl.signedUploadUrl(options, source);
    return unwrapErr(res);
  }

  /**
   * Generate an external URL to allow downloading an object from the bucket.
   *
   * Anyone with possession of the URL can download the given object without
   * any additional auth.
   */
  async signedDownloadUrl(name: string, options?: DownloadUrlOptions): Promise<SignedDownloadUrl> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    const res = await impl.signedDownloadUrl(options, source);
    return unwrapErr(res);
  }

  /**
   * Downloads an object from the bucket and returns its contents.
   */
  async download(name: string, options?: DownloadOptions): Promise<Buffer> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    const res = await impl.downloadAll(options, source);
    return unwrapErr(res);
  }

  /**
   * Removes an object from the bucket.
   * Throws an error on network failure.
   */
  async remove(name: string, options?: DeleteOptions): Promise<void> {
    const source = getCurrentRequest();
    const impl = this.impl.object(name);
    const err = await impl.delete(options, source);
    if (err) {
      unwrapErr(err);
    }
  }

  /**
  * Returns the public URL for accessing the object with the given name.
  * Throws an error if the bucket is not public.
  */
  publicUrl(name: string): string {
    const obj = this.impl.object(name);
    return obj.publicUrl();
  }

  ref<P extends BucketPerms>(): P {
    return this as unknown as P
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
  }
}

export interface UploadUrlOptions {
  /** The expiration time of the url, in seconds from signing. The maximum
   * value is seven days. If no value is given, a default of one hour is
   * used. */
  ttl?: number;
}

export interface SignedUploadUrl {
  url: string;
}

export interface DownloadUrlOptions {
  /** The expiration time of the url, in seconds from signing. The maximum
   * value is seven days. If no value is given, a default of one hour is
   * used. */
  ttl?: number;
}

export interface SignedDownloadUrl {
  url: string;
}
