package gcsemu

import (
	"strconv"
	"strings"
)

type byteRange struct {
	lo, hi, sz int64
}

func parseByteRange(in string) *byteRange {
	var err error
	if !strings.HasPrefix(in, "bytes ") {
		return nil
	}
	in = strings.TrimPrefix(in, "bytes ")
	parts := strings.Split(in, "/")
	if len(parts) != 2 {
		return nil
	}

	ret := byteRange{
		lo: -1,
		hi: -1,
		sz: -1,
	}

	if parts[0] != "*" {
		parts := strings.Split(parts[0], "-")
		if len(parts) != 2 {
			return nil
		}
		ret.lo, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil
		}
		ret.hi, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil
		}
	}

	if parts[1] != "*" {
		ret.sz, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil
		}
	}

	return &ret
}
