package gcsutil

import (
	"testing"

	"gotest.tools/v3/assert"
)

// TODO(dragonsinth): it would be nice to have an integration test that hits real GCS with known data stored.

// TestGcsTokenGen tests that we produce expected GCS tokens that match real data we collected.
func TestGcsTokenGen(t *testing.T) {
	tcs := []struct {
		lastFile string
		cursor   string
	}{
		{
			lastFile: "containers/images/4dcc5142000d12f1a0f67c1e95df4035ca0ebba70117cc04101e53422d391d61/json",
			cursor:   "Cldjb250YWluZXJzL2ltYWdlcy80ZGNjNTE0MjAwMGQxMmYxYTBmNjdjMWU5NWRmNDAzNWNhMGViYmE3MDExN2NjMDQxMDFlNTM0MjJkMzkxZDYxL2pzb24=",
		},
		{
			lastFile: "containers/images/sha256:0e89fc4aeb48f92acff2dddaf610b2ceea5d76a93a44d4c20b31e69d1ed68c10",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6MGU4OWZjNGFlYjQ4ZjkyYWNmZjJkZGRhZjYxMGIyY2VlYTVkNzZhOTNhNDRkNGMyMGIzMWU2OWQxZWQ2OGMxMA==",
		},
		{
			lastFile: "containers/images/sha256:2072bc4567e1f13081af323d046f39453f010471701fa11fc50b786b60512e99",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6MjA3MmJjNDU2N2UxZjEzMDgxYWYzMjNkMDQ2ZjM5NDUzZjAxMDQ3MTcwMWZhMTFmYzUwYjc4NmI2MDUxMmU5OQ==",
		},
		{
			lastFile: "containers/images/sha256:43ecade58b2ddff87a696fada3970491de28c8ea1dca09c988b447b5c5a56412",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6NDNlY2FkZTU4YjJkZGZmODdhNjk2ZmFkYTM5NzA0OTFkZTI4YzhlYTFkY2EwOWM5ODhiNDQ3YjVjNWE1NjQxMg==",
		},
		{
			lastFile: "containers/images/sha256:57076bf87737c2f448e75324ceba121b3b90372ab913f604f928a40da4ebddc7",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6NTcwNzZiZjg3NzM3YzJmNDQ4ZTc1MzI0Y2ViYTEyMWIzYjkwMzcyYWI5MTNmNjA0ZjkyOGE0MGRhNGViZGRjNw==",
		},
		{
			lastFile: "containers/images/sha256:6f73a9b0052f169d8296382660a31050004b691f3e6252008545a3dcb7371a49",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6NmY3M2E5YjAwNTJmMTY5ZDgyOTYzODI2NjBhMzEwNTAwMDRiNjkxZjNlNjI1MjAwODU0NWEzZGNiNzM3MWE0OQ==",
		},
		{
			lastFile: "containers/images/sha256:765b6a129bd04f06c876f3d5b5a346e71a72fae522b230215f67375ccb659a11",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6NzY1YjZhMTI5YmQwNGYwNmM4NzZmM2Q1YjVhMzQ2ZTcxYTcyZmFlNTIyYjIzMDIxNWY2NzM3NWNjYjY1OWExMQ==",
		},
		{
			lastFile: "containers/images/sha256:973d921f391393e65d20a6e990e8ad5aa1129681ab8c54bf59c9192b809594c4",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6OTczZDkyMWYzOTEzOTNlNjVkMjBhNmU5OTBlOGFkNWFhMTEyOTY4MWFiOGM1NGJmNTljOTE5MmI4MDk1OTRjNA==",
		},
		{
			lastFile: "containers/images/sha256:a9d5802ef798d88c0f4f9dc0094249db5e26d8a8a18ba4c2194aab4a44983d2f",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6YTlkNTgwMmVmNzk4ZDg4YzBmNGY5ZGMwMDk0MjQ5ZGI1ZTI2ZDhhOGExOGJhNGMyMTk0YWFiNGE0NDk4M2QyZg==",
		},
		{
			lastFile: "containers/images/sha256:b5714cf3ed3a6b5f1fbc736ef0d5673c5637ccb14d53a23b8728dd828f21a22d",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6YjU3MTRjZjNlZDNhNmI1ZjFmYmM3MzZlZjBkNTY3M2M1NjM3Y2NiMTRkNTNhMjNiODcyOGRkODI4ZjIxYTIyZA==",
		},
		{
			lastFile: "containers/images/sha256:bfdef622d405cb35c466aa29ea7411fd594e7985127bec4e8080572d7ef45cfd",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6YmZkZWY2MjJkNDA1Y2IzNWM0NjZhYTI5ZWE3NDExZmQ1OTRlNzk4NTEyN2JlYzRlODA4MDU3MmQ3ZWY0NWNmZA==",
		},
		{
			lastFile: "containers/images/sha256:da543d0747020a528b8eee057912f6bd07e28f9c006606da28c82c70dac962e2",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6ZGE1NDNkMDc0NzAyMGE1MjhiOGVlZTA1NzkxMmY2YmQwN2UyOGY5YzAwNjYwNmRhMjhjODJjNzBkYWM5NjJlMg==",
		},
		{
			lastFile: "containers/images/sha256:ee73b32b0f0a9dafabd9445a3380094f984858e79c85eafde56c2e037c039c6a",
			cursor:   "Clljb250YWluZXJzL2ltYWdlcy9zaGEyNTY6ZWU3M2IzMmIwZjBhOWRhZmFiZDk0NDVhMzM4MDA5NGY5ODQ4NThlNzljODVlYWZkZTU2YzJlMDM3YzAzOWM2YQ==",
		},
		{
			lastFile: "containers/repositories/library/cpu-test/tag_v1",
			cursor:   "Ci9jb250YWluZXJzL3JlcG9zaXRvcmllcy9saWJyYXJ5L2NwdS10ZXN0L3RhZ192MQ==",
		},
		{
			lastFile: "containers/repositories/library/dns-test/manifest_sha256:9759da4d052f154ba5d0ea32bf0404f442082e3dbad3f3cf6dc6529c6575aec2",
			cursor:   "Cnljb250YWluZXJzL3JlcG9zaXRvcmllcy9saWJyYXJ5L2Rucy10ZXN0L21hbmlmZXN0X3NoYTI1Njo5NzU5ZGE0ZDA1MmYxNTRiYTVkMGVhMzJiZjA0MDRmNDQyMDgyZTNkYmFkM2YzY2Y2ZGM2NTI5YzY1NzVhZWMy",
		},
		{
			lastFile: "containers/repositories/library/dns-test/manifest_sha256:ee49d4935c33260d84499e38dbd5ec3f426c9a2a7e100a901267c712874e4c1d",
			cursor:   "Cnljb250YWluZXJzL3JlcG9zaXRvcmllcy9saWJyYXJ5L2Rucy10ZXN0L21hbmlmZXN0X3NoYTI1NjplZTQ5ZDQ5MzVjMzMyNjBkODQ0OTllMzhkYmQ1ZWMzZjQyNmM5YTJhN2UxMDBhOTAxMjY3YzcxMjg3NGU0YzFk",
		},
		{
			lastFile: "containers/repositories/library/dns-test/tag_v2",
			cursor:   "Ci9jb250YWluZXJzL3JlcG9zaXRvcmllcy9saWJyYXJ5L2Rucy10ZXN0L3RhZ192Mg==",
		},
		{
			lastFile: "containers/repositories/library/kapi-test/manifest_sha256:ca6d9c13fba12363760e6d2495811081b1d2e6fcbf974e551605b65cb5b0a94e",
			cursor:   "Cnpjb250YWluZXJzL3JlcG9zaXRvcmllcy9saWJyYXJ5L2thcGktdGVzdC9tYW5pZmVzdF9zaGEyNTY6Y2E2ZDljMTNmYmExMjM2Mzc2MGU2ZDI0OTU4MTEwODFiMWQyZTZmY2JmOTc0ZTU1MTYwNWI2NWNiNWIwYTk0ZQ==",
		},
		{
			lastFile: "containers/repositories/library/memclient-test/manifest_sha256:7b7979b351c9a019062446c1f033b2f8491868cf943758f006ae219eca231e01",
			cursor:   "Cn9jb250YWluZXJzL3JlcG9zaXRvcmllcy9saWJyYXJ5L21lbWNsaWVudC10ZXN0L21hbmlmZXN0X3NoYTI1Njo3Yjc5NzliMzUxYzlhMDE5MDYyNDQ2YzFmMDMzYjJmODQ5MTg2OGNmOTQzNzU4ZjAwNmFlMjE5ZWNhMjMxZTAx",
		},
	}

	for i, tc := range tcs {
		actualCursor := EncodePageToken(tc.lastFile)
		assert.Equal(t, tc.cursor, actualCursor, "case %d", i)

		lastFile, err := DecodePageToken(actualCursor)
		assert.NilError(t, err, "case %i: failed to decode", i)
		assert.Equal(t, tc.lastFile, lastFile, "case %d", i)
	}
}
