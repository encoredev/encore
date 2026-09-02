package goruntime

import (
	"context"
	"os/exec"

	. "encr.dev/pkg/releaser/bu"
	"github.com/cockroachdb/errors"
)

type CreateTargzInput struct {
	// The directory to tarball.
	SrcDir FSPath
	// The destination of the tarball.
	Dest FSPath
}

func CreateTargz(ctx context.Context, in CreateTargzInput) error {
	cmd := exec.CommandContext(ctx, "tar", "czf", in.Dest.ToIO(), "-C", in.SrcDir.ToIO(), ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "creating tarball: %s", out)
	}
	return nil
}
