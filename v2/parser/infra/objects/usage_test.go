package objects_test

import (
	"testing"

	"encr.dev/v2/parser/infra/objects"
	"encr.dev/v2/parser/resource/usage"
	"encr.dev/v2/parser/resource/usage/usagetest"
)

func TestResolveBucketUsage(t *testing.T) {
	tests := []usagetest.Case{
		{
			Name: "none",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

`,
			Want: []usage.Usage{},
		},
		{
			Name: "upload",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

func Foo() { bkt.Upload(context.Background(), "key") }

`,
			Want: []usage.Usage{&objects.MethodUsage{Method: "Upload", Perm: objects.WriteObject}},
		},
		{
			Name: "sign_upload_url",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

func Foo() { bkt.SignedUploadURL(context.Background(), "key") }

`,
			Want: []usage.Usage{&objects.MethodUsage{Method: "SignedUploadURL", Perm: objects.SignedUploadURL}},
		},
		{
			Name: "ref",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

var ref = objects.BucketRef[objects.Uploader](bkt)
`,
			Want: []usage.Usage{&objects.RefUsage{
				Perms: []objects.Perm{objects.SignedUploadURL, objects.WriteObject},
			}},
		},
		{
			Name: "ref_multi",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

var ref = objects.BucketRef[objects.ReadWriter](bkt)
`,
			Want: []usage.Usage{&objects.RefUsage{
				Perms: []objects.Perm{
					objects.DeleteObject,
					objects.GetObjectMetadata,
					objects.ListObjects,
					objects.ReadObjectContents,
					objects.SignedUploadURL,
					objects.UpdateObjectMetadata,
					objects.WriteObject,
				},
			}},
		},
		{
			Name: "custom_ref_alias",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

type MyRef = objects.Uploader

var ref = objects.BucketRef[MyRef](bkt)
`,
			Want: []usage.Usage{&objects.RefUsage{
				Perms: []objects.Perm{objects.SignedUploadURL, objects.WriteObject},
			}},
		},
		{
			Name: "custom_ref_interface",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

type MyRef interface { objects.Uploader }

var ref = objects.BucketRef[MyRef](bkt)
`,
			Want: []usage.Usage{&objects.RefUsage{
				Perms: []objects.Perm{objects.SignedUploadURL, objects.WriteObject},
			}},
		},
		{
			Name: "custom_ref_interface_multi",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

type MyRef interface { objects.Uploader; objects.Downloader }

var ref = objects.BucketRef[MyRef](bkt)
`,
			Want: []usage.Usage{&objects.RefUsage{
				Perms: []objects.Perm{
					objects.ReadObjectContents,
					objects.SignedUploadURL,
					objects.WriteObject},
			}},
		},
		{
			Name: "invalid_ref",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

type MyRef interface { objects.Uploader; ~int | string; Publish() int }

var ref = objects.BucketRef[MyRef](bkt)
`,
			WantErrs: []string{"Unrecognized permissions in call to objects.BucketRef"},
		},
		{
			Name: "invalid_ref_2",
			Code: `
var bkt = objects.NewBucket("bucket", objects.BucketConfig{})

var ref = objects.BucketRef[string](bkt)
`,
			WantErrs: []string{"Unrecognized permissions in call to objects.BucketRef"},
		},
	}

	usagetest.Run(t, []string{"encore.dev/storage/objects"}, tests)
}
