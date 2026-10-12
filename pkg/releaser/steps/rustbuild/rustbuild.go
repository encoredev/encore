package rustbuild

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	. "encr.dev/pkg/releaser/bu"
	"github.com/cockroachdb/errors"
)

type CompileInput struct {
	// Where the source code is located.
	PackageRoot FSPath

	// The cargo 'target' directory to use for compiled outputs.
	CargoTargetDir FSPath

	// True if it should be built with `--release`.
	ReleaseBuild bool

	ArtifactName    string
	CargoTarget     string
	UseZig          bool
	ZigGlibcVersion string
	ExtraEnvs       []string
}

type CompileOutput struct {
	// Path to the built artifact.
	Artifact FSPath
}

// Compile compiles a rust program.
func Compile(ctx context.Context, in CompileInput) (*CompileOutput, error) {
	// Determine the cargo command line args
	var cargoArgs []string
	if in.UseZig {
		// Make sure cargo-zigbuild is installed.
		if err := installCargoZigbuild(); err != nil {
			return nil, err
		}
		cargoArgs = []string{"zigbuild", "--target", in.CargoTarget + in.ZigGlibcVersion}
	} else {
		cargoArgs = []string{"build", "--target", in.CargoTarget}
	}
	cargoArgs = append(cargoArgs, "--target-dir", in.CargoTargetDir.ToIO())

	buildMode := "debug"
	if in.ReleaseBuild {
		buildMode = "release"
		cargoArgs = append(cargoArgs, "--release")
	}

	cmd := exec.CommandContext(ctx, "cargo", cargoArgs...)
	cmd.Dir = in.PackageRoot.ToIO()
	cmd.Env = append(os.Environ(), in.ExtraEnvs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	CargoLock.Lock()
	defer CargoLock.Unlock()
	err := cmd.Run()
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile native module")
	}

	return &CompileOutput{
		Artifact: in.CargoTargetDir.Join(in.CargoTarget, buildMode, in.ArtifactName),
	}, nil
}

type Artifact struct {
	Executable bool
	Name       string
	Path       FSPath
}

// CargoLock prevents multiple concurrent cargo executions.
var CargoLock sync.Mutex

func installCargoZigbuild() error {
	if _, err := exec.LookPath("cargo-zigbuild"); err == nil {
		return nil
	} else {
		// Check CARGO_HOME
		cargoHome := os.Getenv("CARGO_HOME")
		if cargoHome == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return errors.Wrap(err, "failed to get user home dir")
			}
			cargoHome = filepath.Join(homeDir, ".cargo")
		}
		if _, err := os.Stat(filepath.Join(cargoHome, "bin", "cargo-zigbuild")); err == nil {
			return nil
		}
	}

	CargoLock.Lock()
	defer CargoLock.Unlock()

	cmd := exec.Command("cargo", "install", "cargo-zigbuild")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to install cargo-zigbuild")
	}
	return nil
}
