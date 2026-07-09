package openapi

import (
	"encoding/json"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/getkin/kin-openapi/openapi3"
	"sigs.k8s.io/yaml"
)

func openAPIOperationPatch(doc string) (string, *openapi3.Operation, error) {
	lines := strings.Split(doc, "\n")
	out := make([]string, 0, len(lines))
	var patch *openapi3.Operation

	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "```openapi" {
			out = append(out, lines[i])
			continue
		}

		var block []string
		i++
		for ; i < len(lines) && strings.TrimSpace(lines[i]) != "```"; i++ {
			block = append(block, lines[i])
		}

		p, err := parseOpenAPIOperationPatch(strings.Join(block, "\n"))
		if err != nil {
			return doc, nil, err
		}
		if patch == nil {
			patch = p
		} else {
			mergeOpenAPIOperation(patch, p)
		}
	}

	return strings.TrimSpace(strings.Join(out, "\n")), patch, nil
}

func parseOpenAPIOperationPatch(src string) (*openapi3.Operation, error) {
	b, err := yaml.YAMLToJSON([]byte(src))
	if err != nil {
		return nil, errors.Wrap(err, "parse openapi doc block")
	}

	var op openapi3.Operation
	if err := json.Unmarshal(b, &op); err != nil {
		return nil, errors.Wrap(err, "parse openapi doc block")
	}
	return &op, nil
}

func mergeOpenAPIOperation(dst, src *openapi3.Operation) {
	if len(src.Tags) > 0 {
		dst.Tags = src.Tags
	}
	if src.Summary != "" {
		dst.Summary = src.Summary
	}
	if src.Description != "" {
		dst.Description = src.Description
	}
	if src.OperationID != "" {
		dst.OperationID = src.OperationID
	}
	if len(src.Parameters) > 0 {
		dst.Parameters = mergeOpenAPIParameters(dst.Parameters, src.Parameters)
	}
	if src.RequestBody != nil {
		dst.RequestBody = src.RequestBody
	}
	if src.Responses != nil {
		if dst.Responses == nil {
			dst.Responses = make(openapi3.Responses)
		}
		for code, resp := range src.Responses {
			dst.Responses[code] = resp
		}
	}
	if len(src.Callbacks) > 0 {
		dst.Callbacks = src.Callbacks
	}
	if src.Deprecated {
		dst.Deprecated = true
	}
	if src.Security != nil {
		dst.Security = src.Security
	}
	if src.Servers != nil {
		dst.Servers = src.Servers
	}
	if src.ExternalDocs != nil {
		dst.ExternalDocs = src.ExternalDocs
	}
	if len(src.Extensions) > 0 {
		if dst.Extensions == nil {
			dst.Extensions = make(map[string]any)
		}
		for k, v := range src.Extensions {
			dst.Extensions[k] = v
		}
	}
}

func mergeOpenAPIParameters(dst, src openapi3.Parameters) openapi3.Parameters {
	out := append(openapi3.Parameters{}, dst...)
	for _, p := range src {
		key := openAPIParameterKey(p)
		if key == "" {
			out = append(out, p)
			continue
		}

		replaced := false
		for i, existing := range out {
			if openAPIParameterKey(existing) == key {
				out[i] = p
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, p)
		}
	}
	return out
}

func openAPIParameterKey(p *openapi3.ParameterRef) string {
	if p == nil {
		return ""
	}
	if p.Ref != "" {
		return p.Ref
	}
	if p.Value == nil {
		return ""
	}
	return p.Value.In + ":" + p.Value.Name
}
