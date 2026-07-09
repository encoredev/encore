package openapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPIOperationPatchFromDoc(t *testing.T) {
	c := qt.New(t)

	clean, patch, err := openAPIOperationPatch(`UploadLogo uploads a logo.

More docs.

` + "```openapi" + `
parameters:
  - name: id
    in: path
    required: true
    description: Brand ID.
    schema:
      type: string
requestBody:
  required: true
  content:
    multipart/form-data:
      schema:
        type: object
        required: [logo]
        properties:
          logo:
            type: string
            format: binary
responses:
  "200":
    description: Logo uploaded.
` + "```" + `
`)

	c.Assert(err, qt.IsNil)
	c.Assert(clean, qt.Equals, "UploadLogo uploads a logo.\n\nMore docs.")
	c.Assert(patch.Parameters[0].Value.Description, qt.Equals, "Brand ID.")
	c.Assert(patch.RequestBody.Value.Content.Get("multipart/form-data"), qt.Not(qt.IsNil))
	c.Assert(*patch.Responses["200"].Value.Description, qt.Equals, "Logo uploaded.")
}

func TestOpenAPIOperationPatchErrorsOnUnterminatedBlock(t *testing.T) {
	c := qt.New(t)

	_, _, err := openAPIOperationPatch("Docs.\n\n```openapi\nsummary: Broken")

	c.Assert(err, qt.ErrorMatches, "unterminated ```openapi block.*")
}

func TestMergeOpenAPIOperationReplacesPathParameter(t *testing.T) {
	c := qt.New(t)
	dst := &openapi3.Operation{Parameters: openapi3.Parameters{{Value: &openapi3.Parameter{Name: "id", In: openapi3.ParameterInPath}}}}
	src := &openapi3.Operation{Parameters: openapi3.Parameters{{Value: &openapi3.Parameter{Name: "id", In: openapi3.ParameterInPath, Description: "Brand ID."}}}}

	mergeOpenAPIOperation(dst, src)

	c.Assert(dst.Parameters, qt.HasLen, 1)
	c.Assert(dst.Parameters[0].Value.Description, qt.Equals, "Brand ID.")
}
