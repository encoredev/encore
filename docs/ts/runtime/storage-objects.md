---
title: encore.dev/storage/objects
lang: ts
toc: true
---

# encore.dev/storage/objects

## Classes

### Bucket

Defined in: [storage/objects/bucket.ts:27](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L27)

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

```ts
new Bucket(name, cfg?): Bucket;
```

Defined in: [storage/objects/bucket.ts:34](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L34)

Creates a new bucket with the given name and configuration

###### Parameters

###### name

`string`

###### cfg?

[`BucketConfig`](#bucketconfig)

###### Returns

[`Bucket`](#bucket)

###### Overrides

```ts
BucketPerms.constructor
```

#### Properties

##### impl

```ts
impl: Bucket;
```

Defined in: [storage/objects/bucket.ts:28](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L28)

#### Methods

##### attrs()

```ts
attrs(name, options?): Promise<ObjectAttrs>;
```

Defined in: [storage/objects/bucket.ts:75](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L75)

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

```ts
download(name, options?): Promise<Buffer>;
```

Defined in: [storage/objects/bucket.ts:121](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L121)

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

```ts
exists(name, options?): Promise<boolean>;
```

Defined in: [storage/objects/bucket.ts:64](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L64)

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

```ts
list(options): AsyncGenerator<ListEntry>;
```

Defined in: [storage/objects/bucket.ts:47](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L47)

###### Parameters

###### options

[`ListOptions`](#listoptions)

###### Returns

`AsyncGenerator`\<[`ListEntry`](#listentry)\>

###### Implementation of

[`Lister`](#lister).[`list`](#list-1)

##### publicUrl()

```ts
publicUrl(name): string;
```

Defined in: [storage/objects/bucket.ts:145](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L145)

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

```ts
ref<P>(): P;
```

Defined in: [storage/objects/bucket.ts:150](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L150)

###### Type Parameters

###### P

`P` *extends* [`BucketPerms`](#bucketperms)

###### Returns

`P`

##### remove()

```ts
remove(name, options?): Promise<void>;
```

Defined in: [storage/objects/bucket.ts:132](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L132)

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

```ts
signedDownloadUrl(name, options?): Promise<SignedDownloadUrl>;
```

Defined in: [storage/objects/bucket.ts:111](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L111)

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

```ts
signedUploadUrl(name, options?): Promise<SignedUploadUrl>;
```

Defined in: [storage/objects/bucket.ts:98](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L98)

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

Defined in: [storage/objects/bucket.ts:85](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L85)

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

```ts
static named<name>(name): Bucket;
```

Defined in: [storage/objects/bucket.ts:43](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L43)

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

### ObjectNotFound

Defined in: [storage/objects/error.ts:22](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/error.ts#L22)

#### Extends

- [`ObjectsError`](#objectserror)

#### Constructors

##### Constructor

```ts
new ObjectNotFound(msg): ObjectNotFound;
```

Defined in: [storage/objects/error.ts:23](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/error.ts#L23)

###### Parameters

###### msg

`string`

###### Returns

[`ObjectNotFound`](#objectnotfound)

###### Overrides

[`ObjectsError`](#objectserror).[`constructor`](#constructor-2)

***

### ObjectsError

Defined in: [storage/objects/error.ts:3](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/error.ts#L3)

#### Extends

- `Error`

#### Extended by

- [`ObjectNotFound`](#objectnotfound)
- [`PreconditionFailed`](#preconditionfailed)

#### Constructors

##### Constructor

```ts
new ObjectsError(msg): ObjectsError;
```

Defined in: [storage/objects/error.ts:4](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/error.ts#L4)

###### Parameters

###### msg

`string`

###### Returns

[`ObjectsError`](#objectserror)

###### Overrides

```ts
Error.constructor
```

***

### PreconditionFailed

Defined in: [storage/objects/error.ts:41](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/error.ts#L41)

#### Extends

- [`ObjectsError`](#objectserror)

#### Constructors

##### Constructor

```ts
new PreconditionFailed(msg): PreconditionFailed;
```

Defined in: [storage/objects/error.ts:42](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/error.ts#L42)

###### Parameters

###### msg

`string`

###### Returns

[`PreconditionFailed`](#preconditionfailed)

###### Overrides

[`ObjectsError`](#objectserror).[`constructor`](#constructor-2)

## Interfaces

### Attrser

Defined in: [storage/objects/refs.ts:24](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L24)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### attrs()

```ts
abstract attrs(name, options?): Promise<ObjectAttrs>;
```

Defined in: [storage/objects/refs.ts:25](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L25)

###### Parameters

###### name

`string`

###### options?

[`AttrsOptions`](#attrsoptions)

###### Returns

`Promise`\<[`ObjectAttrs`](#objectattrs)\>

##### exists()

```ts
abstract exists(name, options?): Promise<boolean>;
```

Defined in: [storage/objects/refs.ts:26](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L26)

###### Parameters

###### name

`string`

###### options?

[`ExistsOptions`](#existsoptions)

###### Returns

`Promise`\<`boolean`\>

***

### AttrsOptions

Defined in: [storage/objects/bucket.ts:172](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L172)

Options for retrieving the attributes of an object.

#### Properties

##### version?

```ts
optional version?: string;
```

Defined in: [storage/objects/bucket.ts:179](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L179)

The object version to retrieve attributes for.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

### BucketConfig

Defined in: [storage/objects/bucket.ts:10](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L10)

Configuration options for declaring a Bucket.

#### Properties

##### public?

```ts
optional public?: boolean;
```

Defined in: [storage/objects/bucket.ts:15](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L15)

Whether the objects in the bucket should be publicly
accessible, via CDN. Defaults to false if unset.

##### versioned?

```ts
optional versioned?: boolean;
```

Defined in: [storage/objects/bucket.ts:21](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L21)

Whether to enable versioning of the objects in the bucket.
Defaults to false if unset.

***

### BucketPerms

Defined in: [storage/objects/refs.ts:4](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L4)

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

### DeleteOptions

Defined in: [storage/objects/bucket.ts:198](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L198)

Options for deleting an object from a bucket.

#### Properties

##### version?

```ts
optional version?: string;
```

Defined in: [storage/objects/bucket.ts:205](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L205)

The object version to delete.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

### Downloader

Defined in: [storage/objects/refs.ts:16](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L16)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### download()

```ts
abstract download(name, options?): Promise<Buffer>;
```

Defined in: [storage/objects/refs.ts:17](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L17)

###### Parameters

###### name

`string`

###### options?

[`DownloadOptions`](#downloadoptions)

###### Returns

`Promise`\<`Buffer`\>

***

### DownloadOptions

Defined in: [storage/objects/bucket.ts:211](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L211)

Options for downloading an object from a bucket.

#### Properties

##### version?

```ts
optional version?: string;
```

Defined in: [storage/objects/bucket.ts:218](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L218)

The object version to download.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

### DownloadUrlOptions

Defined in: [storage/objects/bucket.ts:272](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L272)

Options for generating a signed download URL.

#### Properties

##### ttl?

```ts
optional ttl?: number;
```

Defined in: [storage/objects/bucket.ts:276](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L276)

The expiration time of the url, in seconds from signing. The maximum
value is seven days. If no value is given, a default of one hour is
used.

***

### ExistsOptions

Defined in: [storage/objects/bucket.ts:185](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L185)

Options for checking the existence of an object.

#### Properties

##### version?

```ts
optional version?: string;
```

Defined in: [storage/objects/bucket.ts:192](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L192)

The object version to check for existence.
Defaults to the lastest version if unset.

If bucket versioning is not enabled, this option is ignored.

***

### ListEntry

Defined in: [storage/objects/bucket.ts:236](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L236)

A single entry returned when listing objects in a bucket.

#### Properties

##### etag

```ts
etag: string;
```

Defined in: [storage/objects/bucket.ts:239](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L239)

##### name

```ts
name: string;
```

Defined in: [storage/objects/bucket.ts:237](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L237)

##### size

```ts
size: number;
```

Defined in: [storage/objects/bucket.ts:238](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L238)

***

### Lister

Defined in: [storage/objects/refs.ts:29](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L29)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### list()

```ts
abstract list(options): AsyncGenerator<ListEntry>;
```

Defined in: [storage/objects/refs.ts:30](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L30)

###### Parameters

###### options

[`ListOptions`](#listoptions)

###### Returns

`AsyncGenerator`\<[`ListEntry`](#listentry)\>

***

### ListOptions

Defined in: [storage/objects/bucket.ts:158](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L158)

Options for listing objects in a bucket.

#### Properties

##### limit?

```ts
optional limit?: number;
```

Defined in: [storage/objects/bucket.ts:166](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L166)

Maximum number of objects to return. Defaults to no limit.

##### prefix?

```ts
optional prefix?: string;
```

Defined in: [storage/objects/bucket.ts:163](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L163)

Only include objects with this prefix in the listing.
If unset, all objects are included.

***

### ObjectAttrs

Defined in: [storage/objects/bucket.ts:224](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L224)

Describes the attributes of an object stored in a bucket.

#### Properties

##### contentType?

```ts
optional contentType?: string;
```

Defined in: [storage/objects/bucket.ts:230](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L230)

##### etag

```ts
etag: string;
```

Defined in: [storage/objects/bucket.ts:229](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L229)

##### name

```ts
name: string;
```

Defined in: [storage/objects/bucket.ts:225](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L225)

##### size

```ts
size: number;
```

Defined in: [storage/objects/bucket.ts:226](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L226)

##### version?

```ts
optional version?: string;
```

Defined in: [storage/objects/bucket.ts:228](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L228)

The version of the object, if bucket versioning is enabled.

***

### PublicUrler

Defined in: [storage/objects/refs.ts:37](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L37)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### publicUrl()

```ts
abstract publicUrl(name): string;
```

Defined in: [storage/objects/refs.ts:38](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L38)

###### Parameters

###### name

`string`

###### Returns

`string`

***

### Remover

Defined in: [storage/objects/refs.ts:33](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L33)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### remove()

```ts
abstract remove(name, options?): Promise<void>;
```

Defined in: [storage/objects/refs.ts:34](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L34)

###### Parameters

###### name

`string`

###### options?

[`DeleteOptions`](#deleteoptions)

###### Returns

`Promise`\<`void`\>

***

### SignedDownloader

Defined in: [storage/objects/refs.ts:20](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L20)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### signedDownloadUrl()

```ts
abstract signedDownloadUrl(name, options?): Promise<SignedDownloadUrl>;
```

Defined in: [storage/objects/refs.ts:21](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L21)

###### Parameters

###### name

`string`

###### options?

[`DownloadUrlOptions`](#downloadurloptions)

###### Returns

`Promise`\<[`SignedDownloadUrl`](#signeddownloadurl-2)\>

***

### SignedDownloadUrl

Defined in: [storage/objects/bucket.ts:282](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L282)

A signed URL that allows downloading an object without additional auth.

#### Properties

##### url

```ts
url: string;
```

Defined in: [storage/objects/bucket.ts:283](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L283)

***

### SignedUploader

Defined in: [storage/objects/refs.ts:12](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L12)

#### Extends

- [`BucketPerms`](#bucketperms)

#### Methods

##### signedUploadUrl()

```ts
abstract signedUploadUrl(name, options?): Promise<SignedUploadUrl>;
```

Defined in: [storage/objects/refs.ts:13](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L13)

###### Parameters

###### name

`string`

###### options?

[`UploadUrlOptions`](#uploadurloptions)

###### Returns

`Promise`\<[`SignedUploadUrl`](#signeduploadurl-2)\>

***

### SignedUploadUrl

Defined in: [storage/objects/bucket.ts:265](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L265)

A signed URL that allows uploading an object without additional auth.

#### Properties

##### url

```ts
url: string;
```

Defined in: [storage/objects/bucket.ts:266](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L266)

***

### Uploader

Defined in: [storage/objects/refs.ts:8](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L8)

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

Defined in: [storage/objects/refs.ts:9](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L9)

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

### UploadOptions

Defined in: [storage/objects/bucket.ts:245](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L245)

Options for uploading an object to a bucket.

#### Properties

##### contentType?

```ts
optional contentType?: string;
```

Defined in: [storage/objects/bucket.ts:246](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L246)

##### preconditions?

```ts
optional preconditions?: {
  notExists?: boolean;
};
```

Defined in: [storage/objects/bucket.ts:247](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L247)

###### notExists?

```ts
optional notExists?: boolean;
```

***

### UploadUrlOptions

Defined in: [storage/objects/bucket.ts:255](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L255)

Options for generating a signed upload URL.

#### Properties

##### ttl?

```ts
optional ttl?: number;
```

Defined in: [storage/objects/bucket.ts:259](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/bucket.ts#L259)

The expiration time of the url, in seconds from signing. The maximum
value is seven days. If no value is given, a default of one hour is
used.

## Type Aliases

### ReadWriter

```ts
type ReadWriter = Uploader & SignedUploader & Downloader & SignedDownloader & Attrser & Lister & Remover;
```

Defined in: [storage/objects/refs.ts:41](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/storage/objects/refs.ts#L41)
