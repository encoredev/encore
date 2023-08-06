package words

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

func Select(n int) ([]string, error) {
	selected := make([]string, n)
	max := big.NewInt(int64(len(Words)))
	for i := 0; i < n; i++ {
		j, err := rand.Int(rand.Reader, max)
		if err != nil {
			return nil, fmt.Errorf("wordlist.Select %d: %v", n, err)
		}
		selected[i] = Words[j.Int64()]
	}
	return selected, nil
}
