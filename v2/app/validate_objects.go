package app

import (
	"encr.dev/pkg/errors"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/objects"
)

func (d *Desc) validateObjects(pc *parsectx.Context, result *parser.Result) {
	buckets := make(map[string]*objects.Bucket)

	for _, res := range d.Parse.Resources() {
		switch res := res.(type) {
		case *objects.Bucket:
			if existing, ok := buckets[res.Name]; ok {
				pc.Errs.Add(objects.ErrBucketNameNotUnique.
					AtGoNode(existing.AST.Args[0], errors.AsHelp("originally defined here")).
					AtGoNode(res.AST.Args[0], errors.AsError("duplicated here")),
				)
			} else {
				buckets[res.Name] = res
			}

			// Make sure any BucketRef calls are within a service.
			for _, use := range d.Parse.Usages(res) {
				if _, ok := use.(*objects.RefUsage); ok {
					errTxt := "used here"
					if _, ok := d.ServiceForPath(use.DeclaredIn().FSPath); !ok && !use.DeclaredIn().TestFile {
						pc.Errs.Add(objects.ErrBucketRefOutsideService.
							AtGoNode(use, errors.AsError(errTxt)),
						)
					}
				}
			}
		}
	}
}
