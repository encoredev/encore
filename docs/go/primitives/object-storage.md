---
seotitle: Using Object Storage in your backend application
seodesc: Learn how you can use Object Storage to store files and unstructured data in your backend application.
title: Object Storage
subtitle: Simple and scalable storage APIs for files and unstructured data
infobox: {
  title: "Object Storage",
  import: "encore.dev/storage/objects",
}
lang: go
---

Object Storage is a simple and scalable solution to store files and unstructured data in your backend application.

The most common implementation is Amazon S3 ("Simple Storage Service") and its semantics are universally supported by every major cloud provider.

Encore.go provides a cloud-agnostic API for working with Object Storage, allowing you to store and retrieve files with ease. It has support for Amazon S3, Google Cloud Storage, as well as any other S3-compatible implementation (such as DigitalOcean Spaces, MinIO, etc.).

Additionally, when you use Encore's Object Storage API you also automatically get:

* Automatic tracing and instrumentation of all Object Storage operations
* Built-in local development support, storing objects on the local filesystem
* Support for integration testing, using a local, in-memory storage backend

## Creating a Bucket

The core of Object Storage is the **Bucket**, which represents a collection of files.
In Encore, buckets must be declared as package level variables, and cannot be created inside functions.
Regardless of where you create a bucket, it can be accessed from any service by referencing the variable it's assigned to.

When creating a bucket you can configure additional properties, like whether the objects in the bucket should be versioned.

See the complete specification in the [package documentation](https://pkg.go.dev/encore.dev/storage/objects#NewBucket).

For example, to create a bucket for storing profile pictures:

```go
package user

import "encore.dev/storage/objects"

var ProfilePictures = objects.NewBucket("profile-pictures", objects.BucketConfig{
	Versioned: false,
})
```

## Uploading files

To upload a file to a bucket, use the `Upload` method on the bucket variable.
It returns a writer that you can use to write the contents of the file.

To complete the upload, call the `Close` method on the writer.
To abort the upload, either cancel the context or call the `Abort` method on the writer.

The `Upload` method additionally takes a set of options to configure the upload,
like setting attributes (`objects.WithUploadAttrs`) or to reject the upload if the
object already exists (`objects.WithPreconditions`).
See the [package documentation](https://pkg.go.dev/encore.dev/storage/objects#Bucket.Upload) for more details.

```go
package user

import (
		"context"
		"io"
		"net/http"

		"encore.dev/beta/auth"
		"encore.dev/beta/errs"
		"encore.dev/storage/objects"
)

var ProfilePictures = objects.NewBucket("profile-pictures", objects.BucketConfig{})

//encore:api auth raw method=POST path=/upload-profile-picture
func UploadProfilePicture(w http.ResponseWriter, req *http.Request) {
	// Store the user's profile picture with their user id as the key.
	userID, _ := auth.UserID()
	key := string(userID) // We store the profile

	writer := ProfilePictures.Upload(req.Context(), key)
	_, err := io.Copy(writer, req.Body)
	if err != nil {
		// If something went wrong with copying data, abort the upload and return an error.
		writer.Abort()
		errs.HTTPError(w, err)
		return
	}

	if err := writer.Close(); err != nil {
		errs.HTTPError(w, err)
		return
	}

	// All good! Return a 200 OK.
	w.WriteHeader(http.StatusOK)
}
```

## Downloading files

To download a file from a bucket, use the `Download` method on the bucket variable.
It returns a reader that you can use to read the contents of the file.

The `Download` method additionally takes a set of options to configure the download,
like downloading a specific version if the bucket is versioned (`objects.WithVersion`).
See the [package documentation](https://pkg.go.dev/encore.dev/storage/objects#Bucket.Download) for more details.

For example, to download the user's profile picture and serve it:

```go
package user

import (
		"context"
		"io"
		"net/http"

		"encore.dev"
		"encore.dev/beta/auth"
		"encore.dev/beta/errs"
		"encore.dev/storage/objects"
)

var ProfilePictures = objects.NewBucket("profile-pictures", objects.BucketConfig{})

//encore:api public raw method=GET path=/profile-picture/:userID
func ServeProfilePicture(w http.ResponseWriter, req *http.Request) {
	userID := encore.CurrentRequest().PathParams.Get("userID")
	reader := ProfilePictures.Download(req.Context(), userID)

	// Did we encounter an error?
	if err := reader.Err(); err != nil {
		errs.HTTPError(w, err)
		return
	}

	// Assuming all images are JPEGs.
	w.Header().Set("Content-Type", "image/jpeg")
	io.Copy(w, reader)
}
```

## Listing objects

To list objects in a bucket, use the `List` method on the bucket variable.

It returns an iterator of `(error, *objects.ListEntry)` pairs that you can use
to easily iterate over the objects in the bucket using a `range` loop.

For example, to list all profile pictures:

```go
for err, entry := range ProfilePictures.List(ctx, &objects.Query{}) {
	if err != nil {
		// Handle error
	}
	// Do something with entry
}
```

The `*objects.Query` type can be used to limit the number of objects returned,
or to filter them to a specific key prefix.

See the [package documentation](https://pkg.go.dev/encore.dev/storage/objects#Bucket.List) for more details.

## Deleting objects

To delete an object from a bucket, use the `Remove` method on the bucket variable.

For example, to delete a profile picture:

```go
err := ProfilePictures.Remove(ctx, "my-user-id")
if err != nil && !errors.Is(err, objects.ErrObjectNotFound) {
	// Handle error
}
```

## Retrieving object attributes

You can retrieve information about an object using the `Attrs` method on the bucket variable.
It returns the attributes of the object, like its size, content type, and ETag.

For example, to get the attributes of a profile picture:

```go
attrs, err := ProfilePictures.Attrs(ctx, "my-user-id")
if errors.Is(err, objects.ErrObjectNotFound) {
	// Object not found
} else if err != nil {
	// Some other error
}
// Do something with attrs
```

For convenience there is also `Exists` which returns a boolean indicating whether the object exists.

```go
exists, err := ProfilePictures.Exists(ctx, "my-user-id")
if err != nil {
	// Handle error
} else if !exists {
	// Object does not exist
}
```

## Using Public Buckets

Encore supports creating public buckets where objects can be accessed directly via HTTP/HTTPS without authentication. This is useful for serving static assets like images, videos, or other public files.

To create a public bucket, set `Public: true` in the `BucketConfig`:

```go
var PublicAssets = objects.NewBucket("public-assets", objects.BucketConfig{
    Public: true,
})
```

Once configured as public, you can get the public URL for any object using the `PublicURL` method:

```go
// Get the public URL for an object
url := PublicAssets.PublicURL("path/to/image.jpg")

// The URL can be used directly or shared publicly
fmt.Println(url) // e.g. https://assets.example.com/path/to/image.jpg
```
When self-hosting, see how to configure public buckets in the [infrastructure configuration docs](/docs/ts/self-host/configure-infra).

When deploying with Encore Cloud it will automatically configure the bucket to be publicly accessible and [configure CDN](/docs/platform/infrastructure/infra#production-infrastructure) for optimal content delivery.

### Using bucket references

Encore uses static analysis to determine which services are accessing each bucket,
and what operations each service is performing.

That information is used to provision infrastructure correctly,
render architecture diagrams, and configure IAM permissions.

This means that `*objects.Bucket` variables can't be passed around however you'd like,
as it makes static analysis impossible in many cases. To work around these restrictions
Encore allows you to get a "reference" to a bucket that can be passed around any way you want
by calling `objects.BucketRef`.

To ensure Encore still is aware of which permissions each service needs, the call to `objects.BucketRef`
must be made from within a service. Additionally, it must pre-declare the permissions it needs;
those permissions are then assumed to be used by the service.

It looks like this (using the `ProfilePictures` topic above):

```go
ref := objects.BucketRef[objects.Downloader](ProfilePictures)

// ref is of type objects.Downloader, which allows downloading.
```

Encore provides permission interfaces for each operation that can be performed on a bucket:

* `objects.Downloader` for downloading objects
* `objects.Uploader` for uploading objects
* `objects.Lister` for listing objects
* `objects.Attrser` for getting object attributes
* `objects.Remover` for removing objects
* `objects.SignedDownloader` for generating signed download URLs for objects
* `objects.SignedUploader` for generating signed upload URLs for objects

If you need multiple permissions they can be combined by creating an interface
that embeds the permissions you need.

```go
type myPerms interface {
  objects.Downloader
  objects.Uploader
}
ref := objects.BucketRef[myPerms](ProfilePictures)
```

For convenience Encore provides an `objects.ReadWriter` interface that gives complete read-write access
with all the permissions above.

See the [package documentation](https://pkg.go.dev/encore.dev/storage/objects#BucketRef) for more details.

## Signed Upload URLs

You can use `SignedUploadURL` to create signed URLs to allow clients to upload content directly
into the bucket over the internet. The URL is always restricted to one filename, and has a set
expiration date. Anyone in possession of the URL can upload data under this filename without any
additional authentication.

```go
url, err := ProfilePictures.SignedUploadURL(ctx, "my-user-id", objects.WithTTL(time.Duration(7200)*time.Second))
// Pass url to client
```

The client can now `PUT` to this URL with the content as a binary payload.

```bash
curl -X PUT --data-binary @/home/me/dog-wizard.jpeg "https://storage.googleapis.com/profile-pictures/my-user-id/?x-goog-signature=b7a1<...>"
```

### Why signed upload URLs?

Signed URLs are an alternative to accepting the content payload directly in your API. Content
upload requests are sometimes inconvenient to handle well: they can be long running and very large.
With signed URLs, the content flows directly into the storage bucket, and only object IDs and
metadata go through your API service.

The trade-off is that the upload flow becomes more complex from a client point of view.

## Signed Download URLs

You can use `SignedDownloadURL` to create signed URLs to allow clients to download content directly
from the bucket, even if it's private. The URL is always restricted to one filename, and has a set
expiration date. Anyone in possession of the URL can download the file without any additional
authentication.

```go
url, err := Documents.SignedDownloadURL(ctx, "letter-1234", objects.WithTTL(time.Duration(7200)*time.Second))
// Pass url to client
```

### Why signed download URLs?

Similar to the upload case, signed download URLs is a way to avoid handing large files or bulk
traffic through your API. With signed URLs, the content flows directly from the storage bucket,
and only object IDs and metadata go through your API service.

Note: unless the content is private, prefer serving urls with `PublicURL()` over signed URLs.
Public URLs go over CDN, which is typically significantly more performant and cost effective.

