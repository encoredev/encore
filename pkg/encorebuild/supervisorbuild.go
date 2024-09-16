package encorebuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/rs/zerolog"

	"encr.dev/pkg/encorebuild/buildconf"
	. "encr.dev/pkg/encorebuild/buildutil"
	"encr.dev/pkg/encorebuild/compile"
)

func NewSupervisorBuilder(cfg *buildconf.Config) *SupervisorBuilder {
	if cfg.RepoDir == "" {
		Bailf("repo dir not set")
	} else if _, err := os.Stat(cfg.RepoDir); err != nil {
		Bailf("repo does not exist")
	}

	workdir := filepath.Join(cfg.CacheDir, "supervisorbuild", cfg.OS, cfg.Arch)
	Check(os.MkdirAll(workdir, 0755))
	return &SupervisorBuilder{
		log:     cfg.Log,
		cfg:     cfg,
		workdir: workdir,
	}
}

type SupervisorBuilder struct {
	log     zerolog.Logger
	cfg     *buildconf.Config
	workdir string
}

func (b *SupervisorBuilder) Build() {
	b.log.Info().Msgf("Building local Supervisor targeting %s/%s", b.cfg.OS, b.cfg.Arch)
	b.buildRustModule()
	if b.cfg.CopyToRepo {
		b.copyToRepo()
	}
}

// buildRustModule builds the Rust module for the Supervisor runtime.
func (b *SupervisorBuilder) buildRustModule() {
	b.log.Info().Msg("building rust module")
	compile.RustBinary(
		b.cfg,
		"supervisor-encore",
		b.BinaryOutput(),
		filepath.Join(b.cfg.RepoDir, "supervisor"),
		"ENCORE_VERSION="+b.cfg.Version,
		"ENCORE_WORKDIR="+b.workdir,
	)
}

func (b *SupervisorBuilder) copyToRepo() {
	b.log.Info().Msg("copying binary to repo dir")
	copyFile := func(src, dst string) {
		cmd := exec.Command("cp", src, dst)
		out, err := cmd.CombinedOutput()
		if err != nil {
			Bailf("unable to copy binary: %v: %s", err, out)
		}
	}

	src := b.BinaryOutput()
	suffix := ""
	if b.cfg.OS != runtime.GOOS || b.cfg.Arch != runtime.GOARCH {
		suffix = "-" + b.cfg.OS + "-" + b.cfg.Arch
	}
	dst := filepath.Join(b.cfg.RepoDir, "runtimes", "supervisor-encore"+suffix)
	copyFile(src, dst)
}

func (b *SupervisorBuilder) BinaryOutput() string {
	return filepath.Join(b.workdir, "supervisor-encore")
}
