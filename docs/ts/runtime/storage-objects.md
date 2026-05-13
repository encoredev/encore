---
title: encore.dev/storage/objects
lang: ts
toc: true
---

# encore.dev/storage/objects

## Classes

<!-- symbol-start: Bucket -->
### Bucket

<!-- source: storage/objects/bucket.ts:27 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L27)

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

##### Constructor

`new Bucket(name, cfg?): Bucket`

<!-- source: storage/objects/bucket.ts:34 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L34)

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

<!-- source: storage/objects/bucket.ts:28 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L28)

#### Methods

##### attrs()

`attrs(name, options?): Promise<ObjectAttrs>`

<!-- source: storage/objects/bucket.ts:75 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L75)

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

##### download()

`download(name, options?): Promise<Buffer>`

<!-- source: storage/objects/bucket.ts:121 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L121)

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

##### exists()

`exists(name, options?): Promise<boolean>`

<!-- source: storage/objects/bucket.ts:64 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L64)

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

##### list()

`list(options): AsyncGenerator<ListEntry>`

<!-- source: storage/objects/bucket.ts:47 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L47)

###### Parameters

###### options

[`ListOptions`](#listoptions)

###### Returns

`AsyncGenerator`\<[`ListEntry`](#listentry)\>

###### Implementation of

[`Lister`](#lister).[`list`](#list-1)

##### publicUrl()

`publicUrl(name): string`

<!-- source: storage/objects/bucket.ts:145 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L145)

Returns the public URL for accessing the object with the given name.
Throws an error if the bucket is not public.

###### Parameters

###### name

`string`

###### Returns

`string`

###### Implementation of

[`PublicUrler`](#publicurler).[`publicUrl`](#publicurl-1)

##### ref()

`ref<P>(): P`

<!-- source: storage/objects/bucket.ts:150 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L150)

###### Type Parameters

###### P

`P` *extends* [`BucketPerms`](#bucketperms)

###### Returns

`P`

##### remove()

`remove(name, options?): Promise<void>`

<!-- source: storage/objects/bucket.ts:132 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L132)

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

##### signedDownloadUrl()

`signedDownloadUrl(name, options?): Promise<SignedDownloadUrl>`

<!-- source: storage/objects/bucket.ts:111 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L111)

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

##### signedUploadUrl()

`signedUploadUrl(name, options?): Promise<SignedUploadUrl>`

<!-- source: storage/objects/bucket.ts:98 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L98)

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

##### upload()

```ts
upload(
   name, 
   data, 
options?): Promise<ObjectAttrs>;
```

<!-- source: storage/objects/bucket.ts:85 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L85)

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

##### named()

`static named<name>(name): Bucket`

<!-- source: storage/objects/bucket.ts:43 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L43)

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
### ObjectNotFound

<!-- source: storage/objects/error.ts:22 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L22)

#### Extends

- [`ObjectsError`](#objectserror)

#### Constructors

##### Constructor

`new ObjectNotFound(msg): ObjectNotFound`

<!-- source: storage/objects/error.ts:23 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L23)

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
### ObjectsError

<!-- source: storage/objects/error.ts:3 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L3)

#### Extends

- `Error`

#### Extended by

- [`ObjectNotFound`](#objectnotfound)
- [`PreconditionFailed`](#preconditionfailed)

#### Constructors

##### Constructor

`new ObjectsError(msg): ObjectsError`

<!-- source: storage/objects/error.ts:4 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L4)

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
### PreconditionFailed

<!-- source: storage/objects/error.ts:41 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L41)

#### Extends

- [`ObjectsError`](#objectserror)

#### Constructors

##### Constructor

`new PreconditionFailed(msg): PreconditionFailed`

<!-- source: storage/objects/error.ts:42 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/error.ts#L42)

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
### Attrser

<!-- source: storage/objects/refs.ts:24 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L24)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### attrs()

`abstract attrs(name, options?): Promise<ObjectAttrs>`

<!-- source: storage/objects/refs.ts:25 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L25)

###### Parameters

###### name

`string`

###### options?

[`AttrsOptions`](#attrsoptions)

###### Returns

`Promise`\<[`ObjectAttrs`](#objectattrs)\>

##### exists()

`abstract exists(name, options?): Promise<boolean>`

<!-- source: storage/objects/refs.ts:26 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L26)

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
### AttrsOptions

<!-- source: storage/objects/bucket.ts:172 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L172)

Options for retrieving the attributes of an object.

#### Properties

##### version?

`optional version?: string`

<!-- source: storage/objects/bucket.ts:179 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L179)

The object version to retrieve attributes for.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

<!-- symbol-end -->

<!-- symbol-start: BucketConfig -->
### BucketConfig

<!-- source: storage/objects/bucket.ts:10 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L10)

Configuration options for declaring a Bucket.

#### Properties

##### public?

`optional public?: boolean`

<!-- source: storage/objects/bucket.ts:15 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L15)

Whether the objects in the bucket should be publicly
accessible, via CDN. Defaults to false if unset.

##### versioned?

`optional versioned?: boolean`

<!-- source: storage/objects/bucket.ts:21 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L21)

Whether to enable versioning of the objects in the bucket.
Defaults to false if unset.

***

<!-- symbol-end -->

<!-- symbol-start: BucketPerms -->
### BucketPerms

<!-- source: storage/objects/refs.ts:4 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L4)

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
### DeleteOptions

<!-- source: storage/objects/bucket.ts:198 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L198)

Options for deleting an object from a bucket.

#### Properties

##### version?

`optional version?: string`

<!-- source: storage/objects/bucket.ts:205 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L205)

The object version to delete.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

<!-- symbol-end -->

<!-- symbol-start: Downloader -->
### Downloader

<!-- source: storage/objects/refs.ts:16 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L16)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### download()

`abstract download(name, options?): Promise<Buffer>`

<!-- source: storage/objects/refs.ts:17 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L17)

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
### DownloadOptions

<!-- source: storage/objects/bucket.ts:211 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L211)

Options for downloading an object from a bucket.

#### Properties

##### version?

`optional version?: string`

<!-- source: storage/objects/bucket.ts:218 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L218)

The object version to download.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

<!-- symbol-end -->

<!-- symbol-start: DownloadUrlOptions -->
### DownloadUrlOptions

<!-- source: storage/objects/bucket.ts:272 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L272)

Options for generating a signed download URL.

#### Properties

##### ttl?

`optional ttl?: number`

<!-- source: storage/objects/bucket.ts:276 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L276)

The expiration time of the url, in seconds from signing. The maximum
value is seven days. If no value is given, a default of one hour is
used.

***

<!-- symbol-end -->

<!-- symbol-start: ExistsOptions -->
### ExistsOptions

<!-- source: storage/objects/bucket.ts:185 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L185)

Options for checking the existence of an object.

#### Properties

##### version?

`optional version?: string`

<!-- source: storage/objects/bucket.ts:192 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L192)

The object version to check for existence.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

<!-- symbol-end -->

<!-- symbol-start: ListEntry -->
### ListEntry

<!-- source: storage/objects/bucket.ts:236 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L236)

A single entry returned when listing objects in a bucket.

#### Properties

##### etag

`etag: string`

<!-- source: storage/objects/bucket.ts:239 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L239)

##### name

`name: string`

<!-- source: storage/objects/bucket.ts:237 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L237)

##### size

`size: number`

<!-- source: storage/objects/bucket.ts:238 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L238)

***

<!-- symbol-end -->

<!-- symbol-start: Lister -->
### Lister

<!-- source: storage/objects/refs.ts:29 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L29)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### list()

`abstract list(options): AsyncGenerator<ListEntry>`

<!-- source: storage/objects/refs.ts:30 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L30)

###### Parameters

###### options

[`ListOptions`](#listoptions)

###### Returns

`AsyncGenerator`\<[`ListEntry`](#listentry)\>

***

<!-- symbol-end -->

<!-- symbol-start: ListOptions -->
### ListOptions

<!-- source: storage/objects/bucket.ts:158 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L158)

Options for listing objects in a bucket.

#### Properties

##### limit?

`optional limit?: number`

<!-- source: storage/objects/bucket.ts:166 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L166)

Maximum number of objects to return. Defaults to no limit.

##### prefix?

`optional prefix?: string`

<!-- source: storage/objects/bucket.ts:163 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L163)

Only include objects with this prefix in the listing.
If unset, all objects are included.

***

<!-- symbol-end -->

<!-- symbol-start: ObjectAttrs -->
### ObjectAttrs

<!-- source: storage/objects/bucket.ts:224 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L224)

Describes the attributes of an object stored in a bucket.

#### Properties

##### contentType?

`optional contentType?: string`

<!-- source: storage/objects/bucket.ts:230 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L230)

##### etag

`etag: string`

<!-- source: storage/objects/bucket.ts:229 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L229)

##### name

`name: string`

<!-- source: storage/objects/bucket.ts:225 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L225)

##### size

`size: number`

<!-- source: storage/objects/bucket.ts:226 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L226)

##### version?

`optional version?: string`

<!-- source: storage/objects/bucket.ts:228 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L228)

The version of the object, if bucket versioning is enabled.

***

<!-- symbol-end -->

<!-- symbol-start: PublicUrler -->
### PublicUrler

<!-- source: storage/objects/refs.ts:37 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L37)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### publicUrl()

`abstract publicUrl(name): string`

<!-- source: storage/objects/refs.ts:38 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L38)

###### Parameters

###### name

`string`

###### Returns

`string`

***

<!-- symbol-end -->

<!-- symbol-start: Remover -->
### Remover

<!-- source: storage/objects/refs.ts:33 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L33)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### remove()

`abstract remove(name, options?): Promise<void>`

<!-- source: storage/objects/refs.ts:34 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L34)

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
### SignedDownloader

<!-- source: storage/objects/refs.ts:20 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L20)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### signedDownloadUrl()

`abstract signedDownloadUrl(name, options?): Promise<SignedDownloadUrl>`

<!-- source: storage/objects/refs.ts:21 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L21)

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
### SignedDownloadUrl

<!-- source: storage/objects/bucket.ts:282 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L282)

A signed URL that allows downloading an object without additional auth.

#### Properties

##### url

`url: string`

<!-- source: storage/objects/bucket.ts:283 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L283)

***

<!-- symbol-end -->

<!-- symbol-start: SignedUploader -->
### SignedUploader

<!-- source: storage/objects/refs.ts:12 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L12)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### signedUploadUrl()

`abstract signedUploadUrl(name, options?): Promise<SignedUploadUrl>`

<!-- source: storage/objects/refs.ts:13 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L13)

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
### SignedUploadUrl

<!-- source: storage/objects/bucket.ts:265 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L265)

A signed URL that allows uploading an object without additional auth.

#### Properties

##### url

`url: string`

<!-- source: storage/objects/bucket.ts:266 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L266)

***

<!-- symbol-end -->

<!-- symbol-start: Uploader -->
### Uploader

<!-- source: storage/objects/refs.ts:8 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L8)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### upload()

```ts
abstract upload(
   name, 
   data, 
options?): Promise<ObjectAttrs>;
```

<!-- source: storage/objects/refs.ts:9 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L9)

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
### UploadOptions

<!-- source: storage/objects/bucket.ts:245 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L245)

Options for uploading an object to a bucket.

#### Properties

##### contentType?

`optional contentType?: string`

<!-- source: storage/objects/bucket.ts:246 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L246)

##### preconditions?

```ts
optional preconditions?: {
  notExists?: boolean;
};
```

<!-- source: storage/objects/bucket.ts:247 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L247)

###### notExists?

`optional notExists?: boolean`

***

<!-- symbol-end -->

<!-- symbol-start: UploadUrlOptions -->
### UploadUrlOptions

<!-- source: storage/objects/bucket.ts:255 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L255)

Options for generating a signed upload URL.

#### Properties

##### ttl?

`optional ttl?: number`

<!-- source: storage/objects/bucket.ts:259 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/bucket.ts#L259)

The expiration time of the url, in seconds from signing. The maximum
value is seven days. If no value is given, a default of one hour is
used.

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: ReadWriter -->
### ReadWriter

`type ReadWriter = Uploader & SignedUploader & Downloader & SignedDownloader & Attrser & Lister & Remover`

<!-- source: storage/objects/refs.ts:41 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/objects/refs.ts#L41)


<!-- symbol-end -->