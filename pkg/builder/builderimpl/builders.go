package builderimpl

import (
	"encr.dev/pkg/builder"
	"encr.dev/pkg/experiments"
	"encr.dev/v2/v2builder"
)

func Resolve(expSet *experiments.Set) builder.Impl {
	return v2builder.BuilderImpl{}
}
