package pubsub

import (
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/nsqio/go-nsq"
	"github.com/nsqio/nsq/nsqd"
	"github.com/rs/zerolog"
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
	p, err := nsq.NewProducer(n.Addr(), nsq.NewConfig())
	p.SetLogger(&logAdapter{"nsq producer"}, nsq.LogLevelWarning)
	if err != nil {
		return err
	}
	err = p.Ping()
	p.Stop()
	n.nsqd.GetError()
	return err
}

func (n *NSQDaemon) Addr() string {
	return n.nsqd.RealTCPAddr().String()
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

			n.Opts.LogLevel = nsqd.LOG_WARN
			n.Opts.Logger = &logAdapter{"nsqd"}

			// Take the default address options and scope down to localhost (to prevent firewall warnings / permission requests)
			// then set the port to 0 to allow any port to be used which is free
			n.Opts.TCPAddress = "127.0.0.1:0"
			n.Opts.HTTPAddress = "127.0.0.1:0"
			n.Opts.HTTPSAddress = "127.0.0.1:0"
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

type logAdapter struct{ serviceName string }

var _ nsqd.Logger = (*logAdapter)(nil)

func (l *logAdapter) Output(maxdepth int, s string) error {
	// Attempt to extract the level, start with cutting on ":"
	lvl, logMsg, found := strings.Cut(s, ":")
	if !found || strings.Contains(lvl, " ") {
		// then if that fails or we have a space in that cut, try cutting on the first space
		newLvl, suffix, _ := strings.Cut(lvl, " ")
		lvl = newLvl

		if found {
			logMsg = suffix + ":" + logMsg
		}
	}

	// Attempt to convert the level string to a zerolog level
	logLevel := l.OutputLevel(lvl)
	if logLevel == zerolog.NoLevel {
		// and if that fails, then just log the message
		logMsg = s
	}

	log.WithLevel(logLevel).Str("service", l.serviceName).Msg(strings.TrimSpace(logMsg))

	return nil
}

func (l *logAdapter) OutputLevel(lvl string) zerolog.Level {
	switch strings.ToLower(lvl) {
	case "debug", "dbg":
		return zerolog.DebugLevel
	case "info", "inf":
		return zerolog.InfoLevel
	case "warn", "wrn":
		return zerolog.WarnLevel
	case "error", "err":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	default:
		log.Warn().Msg("unknown level: " + lvl)
		return zerolog.NoLevel
	}
}
