package gcsupload

import (
	"context"
	"io"
	"os"

	"cloud.google.com/go/storage"
	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	. "encr.dev/pkg/releaser/bu"
)

type UploadInput struct {
	Bucket  *storage.BucketHandle
	Entries Entries
}

func Upload(ctx context.Context, in UploadInput) error {
	logWriter := zerolog.NewConsoleWriter()
	log := zerolog.New(logWriter).With().Timestamp().Logger()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(20)

	walkEntries(in.Entries, func(e fileEntry) {
		g.Go(func() (err error) {
			log.Info().Msgf("uploading %s to %s", e.Source, e.Dest)
			f, err := os.Open(e.Source.ToIO())
			if err != nil {
				log.Error().Err(err).Msgf("unable to upload %s", e.Dest)
				return errors.Wrapf(err, "open %s", e.Source)
			}
			defer f.Close()

			w := in.Bucket.Object(e.Dest.String()).NewWriter(ctx)
			_, err = io.Copy(w, f)
			if err2 := w.Close(); err == nil {
				err = err2
			}
			if err == nil {
				log.Info().Msgf("successfully uploaded %s", e.Dest)
			} else {
				log.Error().Err(err).Msgf("unable to upload %s", e.Dest)
			}
			return err
		})
	})

	return errors.Wrap(g.Wait(), "upload files to gcs")
}

type Entries []Entry

type Entry interface {
	entry()
}

type File struct {
	Name   string // base name
	Source FSPath
}

type Dir struct {
	Name    string // base name
	Entries []Entry
}

func (File) entry() {}
func (Dir) entry()  {}

type fileEntry struct {
	Source FSPath
	Dest   RelSlashPath
}

type walkFilesFunc func(e fileEntry)

func walkEntries(entries []Entry, fn walkFilesFunc) {
	var walk func(base RelSlashPath, entries []Entry)
	walk = func(base RelSlashPath, entries []Entry) {
		for _, e := range entries {
			switch e := e.(type) {
			case File:
				fn(fileEntry{Source: e.Source, Dest: base.Join(e.Name)})
			case Dir:
				walk(base.Join(e.Name), e.Entries)
			}
		}
	}

	walk("", entries)
}

// MustReadDir reads all files in a directory and returns them as entries.
// It panics if the directory cannot be read.
func MustReadDir(dir FSPath) []Entry {
	entries, err := os.ReadDir(dir.ToIO())
	if err != nil {
		panic(errors.Wrap(err, "read directory"))
	}
	var result []Entry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		result = append(result, File{
			Name:   e.Name(),
			Source: dir.Join(e.Name()),
		})
	}
	return result
}
