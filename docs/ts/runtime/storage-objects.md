---
title: encore.dev/storage/objects
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Bucket -->
### Bucket <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L27" target="_blank" rel="noopener">source</a>

Defines a new Object Storage bucket infrastructure resource.

#### Extends

- [`BucketPerms`](#bucketperms)

#### Implements

- [`Uploader`](#uploader)
- [`SignedUploader`](#signeduploader)
- [`Downloader`](#downloader)
- [`SignedDownloader`](#signeddownloader)
- [`Attrser`](#attrser)
- [`Lister`](#lister)
- [`Remover`](#remover)
- [`PublicUrler`](#publicurler)

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L34" target="_blank" rel="noopener">source</a>

`new Bucket(name, cfg?): Bucket`

Creates a new bucket with the given name and configuration

###### Parameters

###### name

`string`

###### cfg?

[`BucketConfig`](#bucketconfig)

###### Returns

[`Bucket`](#bucket)

###### Overrides

`BucketPerms.constructor`

#### Properties

##### impl

`impl: Bucket`

#### Methods

##### attrs() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L75" target="_blank" rel="noopener">source</a>

`attrs(name, options?): Promise<ObjectAttrs>`

Returns the object's attributes.
Throws an error if the object does not exist.

###### Parameters

###### name

`string`

###### options?

[`AttrsOptions`](#attrsoptions)

###### Returns

`Promise`\<[`ObjectAttrs`](#objectattrs)\>

###### Implementation of

[`Attrser`](#attrser).[`attrs`](#attrs-1)

##### download() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L121" target="_blank" rel="noopener">source</a>

`download(name, options?): Promise<Buffer>`

Downloads an object from the bucket and returns its contents.

###### Parameters

###### name

`string`

###### options?

[`DownloadOptions`](#downloadoptions)

###### Returns

`Promise`\<`Buffer`\>

###### Implementation of

[`Downloader`](#downloader).[`download`](#download-1)

##### exists() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L64" target="_blank" rel="noopener">source</a>

`exists(name, options?): Promise<boolean>`

Returns whether the object exists in the bucket.
Throws an error on network failure.

###### Parameters

###### name

`string`

###### options?

[`ExistsOptions`](#existsoptions)

###### Returns

`Promise`\<`boolean`\>

###### Implementation of

[`Attrser`](#attrser).[`exists`](#exists-1)

##### list() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L47" target="_blank" rel="noopener">source</a>

`list(options): AsyncGenerator<ListEntry>`

###### Parameters

###### options

[`ListOptions`](#listoptions)

###### Returns

`AsyncGenerator`\<[`ListEntry`](#listentry)\>

###### Implementation of

[`Lister`](#lister).[`list`](#list-1)

##### publicUrl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L145" target="_blank" rel="noopener">source</a>

`publicUrl(name): string`

Returns the public URL for accessing the object with the given name.
Throws an error if the bucket is not public.

###### Parameters

###### name

`string`

###### Returns

`string`

###### Implementation of

[`PublicUrler`](#publicurler).[`publicUrl`](#publicurl-1)

##### ref() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L150" target="_blank" rel="noopener">source</a>

`ref<P>(): P`

###### Type Parameters

###### P

`P` *extends* [`BucketPerms`](#bucketperms)

###### Returns

`P`

##### remove() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L132" target="_blank" rel="noopener">source</a>

`remove(name, options?): Promise<void>`

Removes an object from the bucket.
Throws an error on network failure.

###### Parameters

###### name

`string`

###### options?

[`DeleteOptions`](#deleteoptions)

###### Returns

`Promise`\<`void`\>

###### Implementation of

[`Remover`](#remover).[`remove`](#remove-1)

##### signedDownloadUrl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L111" target="_blank" rel="noopener">source</a>

`signedDownloadUrl(name, options?): Promise<SignedDownloadUrl>`

Generate an external URL to allow downloading an object from the bucket.

Anyone with possession of the URL can download the given object without
any additional auth.

###### Parameters

###### name

`string`

###### options?

[`DownloadUrlOptions`](#downloadurloptions)

###### Returns

`Promise`\<[`SignedDownloadUrl`](#signeddownloadurl-2)\>

###### Implementation of

[`SignedDownloader`](#signeddownloader).[`signedDownloadUrl`](#signeddownloadurl-1)

##### signedUploadUrl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L98" target="_blank" rel="noopener">source</a>

`signedUploadUrl(name, options?): Promise<SignedUploadUrl>`

Generate an external URL to allow uploading an object to the bucket.

Anyone with possession of the URL can write to the given object name
without any additional auth.

###### Parameters

###### name

`string`

###### options?

[`UploadUrlOptions`](#uploadurloptions)

###### Returns

`Promise`\<[`SignedUploadUrl`](#signeduploadurl-2)\>

###### Implementation of

[`SignedUploader`](#signeduploader).[`signedUploadUrl`](#signeduploadurl-1)

##### upload() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L85" target="_blank" rel="noopener">source</a>

```ts
upload(
   name, 
   data, 
options?): Promise<ObjectAttrs>;
```

Uploads an object to the bucket.

###### Parameters

###### name

`string`

###### data

`Buffer`

###### options?

[`UploadOptions`](#uploadoptions)

###### Returns

`Promise`\<[`ObjectAttrs`](#objectattrs)\>

###### Implementation of

[`Uploader`](#uploader).[`upload`](#upload-1)

##### named() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L43" target="_blank" rel="noopener">source</a>

`static named<name>(name): Bucket`

Reference an existing bucket by name.
To create a new storage bucket, use `new StorageBucket(...)` instead.

###### Type Parameters

###### name

`name` *extends* `string`

###### Parameters

###### name

`StringLiteral`\<`name`\>

###### Returns

[`Bucket`](#bucket)

***

<!-- symbol-end -->

<!-- symbol-start: ObjectNotFound -->
### ObjectNotFound <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L22" target="_blank" rel="noopener">source</a>

#### Extends

- [`ObjectsError`](#objectserror)

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L23" target="_blank" rel="noopener">source</a>

`new ObjectNotFound(msg): ObjectNotFound`

###### Parameters

###### msg

`string`

###### Returns

[`ObjectNotFound`](#objectnotfound)

###### Overrides

[`ObjectsError`](#objectserror).[`constructor`](#constructor-2)

***

<!-- symbol-end -->

<!-- symbol-start: ObjectsError -->
### ObjectsError <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L3" target="_blank" rel="noopener">source</a>

#### Extends

- `Error`

#### Extended by

- [`ObjectNotFound`](#objectnotfound)
- [`PreconditionFailed`](#preconditionfailed)

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L4" target="_blank" rel="noopener">source</a>

`new ObjectsError(msg): ObjectsError`

###### Parameters

###### msg

`string`

###### Returns

[`ObjectsError`](#objectserror)

###### Overrides

`Error.constructor`

***

<!-- symbol-end -->

<!-- symbol-start: PreconditionFailed -->
### PreconditionFailed <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L41" target="_blank" rel="noopener">source</a>

#### Extends

- [`ObjectsError`](#objectserror)

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L42" target="_blank" rel="noopener">source</a>

`new PreconditionFailed(msg): PreconditionFailed`

###### Parameters

###### msg

`string`

###### Returns

[`PreconditionFailed`](#preconditionfailed)

###### Overrides

[`ObjectsError`](#objectserror).[`constructor`](#constructor-2)

<!-- symbol-end -->

## Interfaces

<!-- symbol-start: Attrser -->
### Attrser <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L24" target="_blank" rel="noopener">source</a>

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### attrs() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L25" target="_blank" rel="noopener">source</a>

`abstract attrs(name, options?): Promise<ObjectAttrs>`

###### Parameters

###### name

`string`

###### options?

[`AttrsOptions`](#attrsoptions)

###### Returns

`Promise`\<[`ObjectAttrs`](#objectattrs)\>

##### exists() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L26" target="_blank" rel="noopener">source</a>

`abstract exists(name, options?): Promise<boolean>`

###### Parameters

###### name

`string`

###### options?

[`ExistsOptions`](#existsoptions)

###### Returns

`Promise`\<`boolean`\>

***

<!-- symbol-end -->

<!-- symbol-start: AttrsOptions -->
### AttrsOptions <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L172" target="_blank" rel="noopener">source</a>

Options for retrieving the attributes of an object.

#### Properties

##### version?

`optional version?: string`

The object version to retrieve attributes for.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

<!-- symbol-end -->

<!-- symbol-start: BucketConfig -->
### BucketConfig <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L10" target="_blank" rel="noopener">source</a>

Configuration options for declaring a Bucket.

#### Properties

##### public?

`optional public?: boolean`

Whether the objects in the bucket should be publicly
accessible, via CDN. Defaults to false if unset.

##### versioned?

`optional versioned?: boolean`

Whether to enable versioning of the objects in the bucket.
Defaults to false if unset.

***

<!-- symbol-end -->

<!-- symbol-start: BucketPerms -->
### BucketPerms <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L4" target="_blank" rel="noopener">source</a>

#### Extended by

- [`Bucket`](#bucket)
- [`Uploader`](#uploader)
- [`SignedUploader`](#signeduploader)
- [`Downloader`](#downloader)
- [`SignedDownloader`](#signeddownloader)
- [`Attrser`](#attrser)
- [`Lister`](#lister)
- [`PublicUrler`](#publicurler)
- [`Remover`](#remover)

***

<!-- symbol-end -->

<!-- symbol-start: DeleteOptions -->
### DeleteOptions <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L198" target="_blank" rel="noopener">source</a>

Options for deleting an object from a bucket.

#### Properties

##### version?

`optional version?: string`

The object version to delete.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

<!-- symbol-end -->

<!-- symbol-start: Downloader -->
### Downloader <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L16" target="_blank" rel="noopener">source</a>

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### download() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L17" target="_blank" rel="noopener">source</a>

`abstract download(name, options?): Promise<Buffer>`

###### Parameters

###### name

`string`

###### options?

[`DownloadOptions`](#downloadoptions)

###### Returns

`Promise`\<`Buffer`\>

***

<!-- symbol-end -->

<!-- symbol-start: DownloadOptions -->
### DownloadOptions <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L211" target="_blank" rel="noopener">source</a>

Options for downloading an object from a bucket.

#### Properties

##### version?

`optional version?: string`

The object version to download.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

<!-- symbol-end -->

<!-- symbol-start: DownloadUrlOptions -->
### DownloadUrlOptions <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L272" target="_blank" rel="noopener">source</a>

Options for generating a signed download URL.

#### Properties

##### ttl?

`optional ttl?: number`

The expiration time of the url, in seconds from signing. The maximum
value is seven days. If no value is given, a default of one hour is
used.

***

<!-- symbol-end -->

<!-- symbol-start: ExistsOptions -->
### ExistsOptions <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L185" target="_blank" rel="noopener">source</a>

Options for checking the existence of an object.

#### Properties

##### version?

`optional version?: string`

The object version to check for existence.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

<!-- symbol-end -->

<!-- symbol-start: ListEntry -->
### ListEntry <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L236" target="_blank" rel="noopener">source</a>

A single entry returned when listing objects in a bucket.

#### Properties

##### etag

`etag: string`

##### name

`name: string`

##### size

`size: number`

***

<!-- symbol-end -->

<!-- symbol-start: Lister -->
### Lister <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L29" target="_blank" rel="noopener">source</a>

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### list() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L30" target="_blank" rel="noopener">source</a>

`abstract list(options): AsyncGenerator<ListEntry>`

###### Parameters

###### options

[`ListOptions`](#listoptions)

###### Returns

`AsyncGenerator`\<[`ListEntry`](#listentry)\>

***

<!-- symbol-end -->

<!-- symbol-start: ListOptions -->
### ListOptions <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L158" target="_blank" rel="noopener">source</a>

Options for listing objects in a bucket.

#### Properties

##### limit?

`optional limit?: number`

Maximum number of objects to return. Defaults to no limit.

##### prefix?

`optional prefix?: string`

Only include objects with this prefix in the listing.
If unset, all objects are included.

***

<!-- symbol-end -->

<!-- symbol-start: ObjectAttrs -->
### ObjectAttrs <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L224" target="_blank" rel="noopener">source</a>

Describes the attributes of an object stored in a bucket.

#### Properties

##### contentType?

`optional contentType?: string`

##### etag

`etag: string`

##### name

`name: string`

##### size

`size: number`

##### version?

`optional version?: string`

The version of the object, if bucket versioning is enabled.

***

<!-- symbol-end -->

<!-- symbol-start: PublicUrler -->
### PublicUrler <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L37" target="_blank" rel="noopener">source</a>

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### publicUrl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L38" target="_blank" rel="noopener">source</a>

`abstract publicUrl(name): string`

###### Parameters

###### name

`string`

###### Returns

`string`

***

<!-- symbol-end -->

<!-- symbol-start: Remover -->
### Remover <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L33" target="_blank" rel="noopener">source</a>

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### remove() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L34" target="_blank" rel="noopener">source</a>

`abstract remove(name, options?): Promise<void>`

###### Parameters

###### name

`string`

###### options?

[`DeleteOptions`](#deleteoptions)

###### Returns

`Promise`\<`void`\>

***

<!-- symbol-end -->

<!-- symbol-start: SignedDownloader -->
### SignedDownloader <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L20" target="_blank" rel="noopener">source</a>

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### signedDownloadUrl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L21" target="_blank" rel="noopener">source</a>

`abstract signedDownloadUrl(name, options?): Promise<SignedDownloadUrl>`

###### Parameters

###### name

`string`

###### options?

[`DownloadUrlOptions`](#downloadurloptions)

###### Returns

`Promise`\<[`SignedDownloadUrl`](#signeddownloadurl-2)\>

***

<!-- symbol-end -->

<!-- symbol-start: SignedDownloadUrl -->
### SignedDownloadUrl <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L282" target="_blank" rel="noopener">source</a>

A signed URL that allows downloading an object without additional auth.

#### Properties

##### url

`url: string`

***

<!-- symbol-end -->

<!-- symbol-start: SignedUploader -->
### SignedUploader <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L12" target="_blank" rel="noopener">source</a>

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### signedUploadUrl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L13" target="_blank" rel="noopener">source</a>

`abstract signedUploadUrl(name, options?): Promise<SignedUploadUrl>`

###### Parameters

###### name

`string`

###### options?

[`UploadUrlOptions`](#uploadurloptions)

###### Returns

`Promise`\<[`SignedUploadUrl`](#signeduploadurl-2)\>

***

<!-- symbol-end -->

<!-- symbol-start: SignedUploadUrl -->
### SignedUploadUrl <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L265" target="_blank" rel="noopener">source</a>

A signed URL that allows uploading an object without additional auth.

#### Properties

##### url

`url: string`

***

<!-- symbol-end -->

<!-- symbol-start: Uploader -->
### Uploader <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L8" target="_blank" rel="noopener">source</a>

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### upload() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L9" target="_blank" rel="noopener">source</a>

```ts
abstract upload(
   name, 
   data, 
options?): Promise<ObjectAttrs>;
```

###### Parameters

###### name

`string`

###### data

`Buffer`

###### options?

[`UploadOptions`](#uploadoptions)

###### Returns

`Promise`\<[`ObjectAttrs`](#objectattrs)\>

***

<!-- symbol-end -->

<!-- symbol-start: UploadOptions -->
### UploadOptions <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L245" target="_blank" rel="noopener">source</a>

Options for uploading an object to a bucket.

#### Properties

##### contentType?

`optional contentType?: string`

##### preconditions?

```ts
optional preconditions?: {
  notExists?: boolean;
};
```

###### notExists?

`optional notExists?: boolean`

***

<!-- symbol-end -->

<!-- symbol-start: UploadUrlOptions -->
### UploadUrlOptions <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L255" target="_blank" rel="noopener">source</a>

Options for generating a signed upload URL.

#### Properties

##### ttl?

`optional ttl?: number`

The expiration time of the url, in seconds from signing. The maximum
value is seven days. If no value is given, a default of one hour is
used.

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: ReadWriter -->
### ReadWriter <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L41" target="_blank" rel="noopener">source</a>

`type ReadWriter = Uploader & SignedUploader & Downloader & SignedDownloader & Attrser & Lister & Remover`


<!-- symbol-end -->