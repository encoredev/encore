package pubsub

import (
	"os"

	"github.com/cockroachdb/errors"
	"github.com/nsqio/go-nsq"
	"github.com/nsqio/nsq/nsqd"
	"github.com/rs/zerolog/log"
	"go4.org/syncutil"
)

type NSQDaemon struct {
	nsqd      *nsqd.NSQD
	startOnce syncutil.Once

	Opts *nsqd.Options
}

func (n *NSQDaemon) Stats() (*nsqd.Stats, error) {
	if n.nsqd == nil {
		return nil, errors.New("nsqd not started")
	}
	stats := n.nsqd.GetStats("", "", true)
	return &stats, nil
}

func (n *NSQDaemon) isReady() error {
	p, err := nsq.NewProducer(n.Opts.TCPAddress, nsq.NewConfig())
	if err != nil {
		return err
	}
	err = p.Ping()
	p.Stop()
	n.nsqd.GetError()
	return err
}

func (n *NSQDaemon) Start() error {
	return n.startOnce.Do(func() error {
		if n.Opts == nil {
			n.Opts = nsqd.NewOptions()
			tmpDir, err := os.MkdirTemp("", "encore-nsqd")
			if err != nil {
				return errors.Wrap(err, "failed to create tmp nsqd datapath")
			}
			n.Opts.DataPath = tmpDir
		}
		nsq, err := nsqd.New(n.Opts)
		if err != nil {
			return errors.Wrap(err, "failed to create new nsqd")
		}
		n.nsqd = nsq
		go func() {
			err = nsq.Main()
			if err != nil {
				log.Err(err).Msg("failed to start nsqd")
			}
		}()
		// Ping the daemon to make sure it has started correctly
		return n.isReady()
	})
}

func (n *NSQDaemon) Stop() {
	if n.nsqd != nil {
		n.nsqd.Exit()
	}
}
