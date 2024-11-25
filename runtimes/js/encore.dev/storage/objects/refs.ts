import type { AttrsOptions, DeleteOptions, DownloadOptions, ExistsOptions, ListEntry, ListOptions, ObjectAttrs, UploadOptions } from "./bucket";

export abstract class BucketPerms {
  private bucketPerms(): void { };
}

export abstract class Uploader extends BucketPerms {
  abstract upload(name: string, data: Buffer, options?: UploadOptions): Promise<ObjectAttrs>;
}

export abstract class Downloader extends BucketPerms {
  abstract download(name: string, options?: DownloadOptions): Promise<Buffer>;
}

export abstract class Attrser extends BucketPerms {
  abstract attrs(name: string, options?: AttrsOptions): Promise<ObjectAttrs>;
  abstract exists(name: string, options?: ExistsOptions): Promise<boolean>;
}

export abstract class Lister extends BucketPerms {
  abstract list(options: ListOptions): AsyncGenerator<ListEntry>;
}

export abstract class Remover extends BucketPerms {
  abstract remove(name: string, options?: DeleteOptions): Promise<void>;
}

export abstract class PublicUrler extends BucketPerms {
  abstract publicUrl(name: string): string;
}

export type ReadWriter =
  & Uploader
  & Downloader
  & Attrser
  & Lister
  & Remover;
