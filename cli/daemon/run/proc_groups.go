package run

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/internal/sym"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// ProcGroup represents a running Encore process.
type ProcGroup struct {
	ID          string           // unique process id
	Run         *Run             // the run the process belongs to
	Pid         int              // the OS process id
	Meta        *meta.Data       // app metadata snapshot
	Started     time.Time        // when the process started
	Experiments *experiments.Set // enabled experiments

	ctx      context.Context
	log      zerolog.Logger
	exit     chan struct{} // closed when the process has exited
	cmd      *exec.Cmd
	reqWr    *os.File
	respRd   *os.File
	buildDir string
	Client   *yamux.Session
	authKey  config.EncoreAuthKey

	sym       *sym.Table
	symErr    error
	symParsed chan struct{} // closed when sym and symErr are set
}

// Done returns a channel that is closed when the process has exited.
func (p *ProcGroup) Done() <-chan struct{} {
	return p.exit
}

// Close closes the process and waits for it to shutdown.
// It can safely be called multiple times.
func (p *ProcGroup) Close() {
	p.reqWr.Close()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-p.exit:
	case <-timer.C:
		// The process didn't exit after 10s
		p.log.Error().Msg("timed out waiting for process to exit; killing")
		p.cmd.Process.Kill()
		<-p.exit
	}
}

func (p *ProcGroup) waitForExit() {
	defer close(p.exit)
	defer closeAll(p.reqWr, p.respRd)

	if err := p.cmd.Wait(); err != nil && p.ctx.Err() == nil {
		p.log.Error().Err(err).Msg("process exited with error")
	} else {
		p.log.Info().Msg("process exited successfully")
	}

	// Flush the logs in case the output did not end in a newline.
	for _, w := range [...]io.Writer{p.cmd.Stdout, p.cmd.Stderr} {
		if w != nil {
			w.(*logWriter).Flush()
		}
	}
}

// parseSymTable parses the symbol table of the binary at binPath
// and stores the result in p.sym and p.symErr.
func (p *ProcGroup) parseSymTable(binPath string) {
	parse := func() (*sym.Table, error) {
		f, err := os.Open(binPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return sym.Load(f)
	}

	defer close(p.symParsed)
	p.sym, p.symErr = parse()
}

// SymTable waits for the proc's symbol table to be parsed and then returns it.
// ctx is used to cancel the wait.
func (p *ProcGroup) SymTable(ctx context.Context) (*sym.Table, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.symParsed:
		return p.sym, p.symErr
	}
}
