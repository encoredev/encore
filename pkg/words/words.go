package words

import (
	_ "embed" // for go:embed
	"strings"
	"sync"
)

type wordList struct {
	raw   string
	once  sync.Once
	words []string
}

func (w *wordList) Get() []string {
	w.once.Do(func() {
		raw := strings.TrimSpace(w.raw)
		w.words = strings.Split(raw, "\n")
	})
	return w.words
}

var (
	//go:embed shortwords.txt
	shortRaw string

	shortWords = wordList{raw: shortRaw}
)
